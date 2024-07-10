package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
	"truefoundry/resolver/internal/throttler"

	"github.com/truefoundry/elasti/pkg/messages"
	"go.uber.org/zap"
)

type (
	// Handler is the reverse proxy handler
	Handler struct {
		logger      *zap.Logger
		throttler   *throttler.Throttler
		transport   http.RoundTripper
		bufferPool  httputil.BufferPool
		timeout     time.Duration
		operatorRPC Operator
		hostManager HostManager
	}

	// HandlerParams is the configuration for the handler
	HandlerParams struct {
		Logger      *zap.Logger
		ReqTimeout  time.Duration
		OperatorRPC Operator
		HostManager HostManager
		Throttler   *throttler.Throttler
		Transport   http.RoundTripper
	}

	// Operator is to communicate with the operator
	Operator interface {
		SendIncomingRequestInfo(ns, svc string)
	}

	// HostManager is to manage the hosts, and their traffic
	HostManager interface {
		GetHost(req *http.Request) (*messages.Host, error)
		DisableTrafficForHost(service string)
	}
)

// NewHandler returns a new Handler
func NewHandler(hc *HandlerParams) *Handler {
	return &Handler{
		throttler:   hc.Throttler,
		logger:      hc.Logger.With(zap.String("component", "handler")),
		transport:   hc.Transport,
		bufferPool:  NewBufferPool(),
		timeout:     hc.ReqTimeout,
		operatorRPC: hc.OperatorRPC,
		hostManager: hc.HostManager,
	}
}

type Response struct {
	Message string `json:"message"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Get host details from hostManager
	h.logger.Debug("Request received")
	host, err := h.hostManager.GetHost(req)
	if err != nil {
		h.logger.Error("Error getting host", zap.Error(err))
		http.Error(w, "Error getting host", http.StatusInternalServerError)
		return
	}
	h.logger.Debug("host received", zap.Any("host", host))

	// This closes the connections, in case the host is scaled up by the controller.
	if !host.TrafficAllowed {
		h.logger.Info("Traffic not allowed", zap.Any("host", host))
		w.Header().Set("Connection", "close")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, err := w.Write([]byte(`{"error": "traffic is switched"}`))
		if err != nil {
			h.logger.Error("Error writing response", zap.Error(err))
			return
		}
		return
	}

	// Inform the controller about the incoming request
	go h.operatorRPC.SendIncomingRequestInfo(host.Namespace, host.SourceService)

	// Send request to throttler
	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()
	if tryErr := h.throttler.Try(ctx, host, func(count int) error {
		err := h.ProxyRequest(w, req, host.TargetHost, count)
		if err != nil {
			return err
		}
		h.hostManager.DisableTrafficForHost(host.SourceService)
		return nil
	}); tryErr != nil {
		h.logger.Error("throttler try error: ", zap.Error(tryErr))
		if errors.Is(tryErr, context.DeadlineExceeded) {
			http.Error(w, tryErr.Error(), http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
	h.logger.Debug("Try completed")
}

func (h *Handler) ProxyRequest(w http.ResponseWriter, req *http.Request, targetHost string, count int) (rErr error) {
	defer func() {
		if r := recover(); r != nil {
			rErr = fmt.Errorf("panic in ProxyRequest: %w", r.(error))
		}
	}()
	targetURL, err := url.Parse(targetHost + req.RequestURI)
	if err != nil {
		h.logger.Error("Error parsing target URL", zap.Error(err))
		http.Error(w, "Error parsing target URL", http.StatusInternalServerError)
		return
	}
	//proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy := h.NewHeaderPruningReverseProxy(targetURL, true)
	proxy.BufferPool = h.bufferPool
	proxy.Transport = h.transport
	// req.Header.Set("elasti-retry-count", strconv.Itoa(count))
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		panic(fmt.Errorf("serveHTTP error: %w", err))
	}
	proxy.ServeHTTP(w, req)
	return nil
}

// NewHeaderPruningReverseProxy returns a httputil.ReverseProxy that proxies
// requests to the given targetHost after removing the headersToRemove.
// If hostOverride is not an empty string, the outgoing request's Host header will be
// replaced with that explicit value and the passthrough loadbalancing header will be
// set to enable pod-addressability.
func (h *Handler) NewHeaderPruningReverseProxy(target *url.URL, hostOverride bool) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL = target

			if hostOverride {
				req.Host = target.Host
			}
		},
	}
}

type bufferPool struct {
	pool *sync.Pool
}

// Get gets a []byte from the bufferPool, or creates a new one if none are
// available in the pool.
func (b *bufferPool) Get() []byte {
	buf := b.pool.Get()
	if buf == nil {
		// Use the default buffer size as defined in the ReverseProxy itself.
		return make([]byte, 32*1024)
	}

	return *buf.(*[]byte)
}

// Put returns the given Buffer to the bufferPool.
func (b *bufferPool) Put(buffer []byte) {
	b.pool.Put(&buffer)
}

func NewBufferPool() httputil.BufferPool {
	return &bufferPool{
		// We don't use the New function of sync.Pool here to avoid an unnecessary
		// allocation when creating the slices. They are implicitly created in the
		// Get function below.
		pool: &sync.Pool{},
	}
}

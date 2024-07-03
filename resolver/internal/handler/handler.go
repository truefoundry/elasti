package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"runtime"
	"strconv"
	"sync"
	"time"
	"truefoundry/resolver/internal/throttler"

	"github.com/truefoundry/elasti/pkg/k8sHelper"
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

	// HandlerConfig is the configuration for the handler
	HandlerConfig struct {
		MaxIdleProxyConns        int
		MaxIdleProxyConnsPerHost int
		OperatorRPC              Operator
		HostManager              HostManager
		K8sUtil                  *k8sHelper.Ops
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
func NewHandler(ctx context.Context, logger *zap.Logger, hc *HandlerConfig) *Handler {
	transport := throttler.NewProxyAutoTransport(logger, hc.MaxIdleProxyConns, hc.MaxIdleProxyConnsPerHost)
	newThrottler := throttler.NewThrottler(ctx, logger, hc.K8sUtil)
	return &Handler{
		throttler:   newThrottler,
		logger:      logger.With(zap.String("component", "handler")),
		transport:   transport,
		bufferPool:  NewBufferPool(),
		timeout:     10 * time.Second,
		operatorRPC: hc.OperatorRPC,
		hostManager: hc.HostManager,
	}
}

type Response struct {
	Message string `json:"message"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	h.logger.Debug("Request received")
	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()
	host, err := h.hostManager.GetHost(req)
	if err != nil {
		h.logger.Error("Error getting host", zap.Error(err))
		http.Error(w, "Error getting host", http.StatusInternalServerError)
		return
	}
	h.logger.Debug("host received", zap.Any("host", host))
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
	go h.operatorRPC.SendIncomingRequestInfo(host.Namespace, host.SourceService)
	select {
	case <-ctx.Done():
		h.logger.Error("Request timeout", zap.Error(ctx.Err()))
		w.WriteHeader(http.StatusInternalServerError)
	default:
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
	}
}

func (h *Handler) ProxyRequest(w http.ResponseWriter, req *http.Request, host string, count int) (rErr error) {
	defer func() {
		if r := recover(); r != nil {
			h.logger.Error("Recovered from panic", zap.Any("panic", r))
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			h.logger.Error("Panic stack trace", zap.ByteString("stacktrace", buf[:n]))
			rErr = r.(error)
		}
	}()
	targetURL, err := url.Parse(host)
	if err != nil {
		h.logger.Error("Error parsing target URL", zap.Error(err))
		http.Error(w, "Error parsing target URL", http.StatusInternalServerError)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.BufferPool = h.bufferPool
	proxy.Transport = h.transport
	req.Header.Set("elasti-retry-count", strconv.Itoa(count))
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		panic(err)
	}
	proxy.ServeHTTP(w, req)
	return nil
}

// NoHostOverride signifies that no host overriding should be done and that the host
// should be inferred from the target of the reverse-proxy.
const NoHostOverride = ""

// NewHeaderPruningReverseProxy returns a httputil.ReverseProxy that proxies
// requests to the given targetHost after removing the headersToRemove.
// If hostOverride is not an empty string, the outgoing request's Host header will be
// replaced with that explicit value and the passthrough loadbalancing header will be
// set to enable pod-addressability.
func (h *Handler) NewHeaderPruningReverseProxy(target, hostOverride string, useHTTPS bool) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			if useHTTPS {
				req.URL.Scheme = "https"
			} else {
				req.URL.Scheme = "http"
			}
			req.URL.Host = target

			if hostOverride != NoHostOverride {
				req.Host = target
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

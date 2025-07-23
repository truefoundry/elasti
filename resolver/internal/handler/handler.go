package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/truefoundry/elasti/resolver/internal/prom"
	"github.com/truefoundry/elasti/resolver/internal/throttler"

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

	// Params is the configuration for the handler
	Params struct {
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
func NewHandler(hc *Params) *Handler {
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

type QueueStatusResponse struct {
	QueueStatus int `json:"queueStatus"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	start := time.Now()
	customWriter := newResponseWriter(w)
	host, err := h.handleAnyRequest(customWriter, req)
	var responseStatus, errorMessage string
	if err != nil {
		errorMessage = err.Error()
	}
	responseStatus = http.StatusText(customWriter.statusCode)
	duration := time.Since(start).Seconds()
	prom.IncomingRequestHistogram.WithLabelValues(
		host.SourceService,
		host.TargetService,
		host.SourceHost,
		host.TargetHost,
		host.Namespace,
		req.Method,
		req.RequestURI,
		responseStatus,
		errorMessage,
	).Observe(duration)
}

// handleAnyRequest handles any incoming request
func (h *Handler) handleAnyRequest(w http.ResponseWriter, req *http.Request) (*messages.Host, error) {
	host, err := h.hostManager.GetHost(req)
	if err != nil {
		http.Error(w, "Error getting host", http.StatusInternalServerError)
		h.logger.Error("error getting host", zap.Error(err))
		return host, fmt.Errorf("error getting host: %w", err)
	}
	h.logger.Debug("request received", zap.Any("host", host))

	prom.QueuedRequestGauge.WithLabelValues(host.SourceService, host.Namespace).Inc()
	defer prom.QueuedRequestGauge.WithLabelValues(host.SourceService, host.Namespace).Dec()

	// This closes the connections, in case the host is scaled up by the controller.
	if !host.TrafficAllowed {
		h.logger.Info("Traffic not allowed", zap.Any("host", host))
		w.Header().Set("Connection", "close")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, err := w.Write([]byte(`{"error": "traffic is switched"}`))
		if err != nil {
			h.logger.Error("Error writing response", zap.Error(err))
			return host, fmt.Errorf("error writing response: %w", err)
		}
		return host, fmt.Errorf("traffic not allowed by resolver")
	}

	// Inform the controller about the incoming request
	go h.operatorRPC.SendIncomingRequestInfo(host.Namespace, host.SourceService)

	// Send request to throttler
	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()
	if tryErr := h.throttler.Try(ctx, host,
		func(count int) error {
			err := h.ProxyRequest(w, req, host, count)
			if err != nil {
				h.logger.Error("Error proxying request", zap.Error(err))
				hub := sentry.GetHubFromContext(req.Context())
				hub.CaptureException(err)
				return err
			}
			h.hostManager.DisableTrafficForHost(host.IncomingHost)
			return nil
		}, func() {
			h.operatorRPC.SendIncomingRequestInfo(host.Namespace, host.SourceService)
		}); tryErr != nil {
		h.logger.Error("throttler try error: ", zap.Error(tryErr))
		hub := sentry.GetHubFromContext(req.Context())
		if hub != nil {
			hub.CaptureException(tryErr)
		}

		if errors.Is(tryErr, context.DeadlineExceeded) {
			http.Error(w, "request timeout", http.StatusRequestTimeout)
			return host, fmt.Errorf("throttler try error: %w", tryErr)
		}
		w.WriteHeader(http.StatusInternalServerError)
		return host, fmt.Errorf("throttler try error: %w", tryErr)
	}
	return host, nil
}

func (h *Handler) ProxyRequest(w http.ResponseWriter, req *http.Request, host *messages.Host, count int) (rErr error) {
	defer func() {
		if r := recover(); r != nil {
			rErr = fmt.Errorf("panic in ProxyRequest: %w", r.(error))
		}
	}()
	targetURL, err := url.Parse(host.TargetHost + req.RequestURI)
	if err != nil {
		return fmt.Errorf("error parsing target URL: %w", err)
	}

	// Pass false to hostOverride to preserve the original host
	proxy := h.NewHeaderPruningReverseProxy(targetURL, false)
	proxy.BufferPool = h.bufferPool
	proxy.Transport = h.transport
	proxy.ErrorHandler = func(wErr http.ResponseWriter, reqErr *http.Request, err error) {
		h.logger.Error("reverse proxy error", zap.Error(err), zap.String("url", reqErr.URL.String()))
		if wErr.Header().Get("Content-Type") == "" {
			wErr.Header().Set("Content-Type", "text/plain; charset=utf-8")
			wErr.WriteHeader(http.StatusBadGateway)
			_, err = wErr.Write([]byte("Bad Gateway"))
			if err != nil {
				h.logger.Error("error writing response", zap.Error(err))
			}
		}
	}
	h.logger.Info("Request proxied", zap.Int("Retry Count", count))
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
			originalHost := req.Host // Save the original host
			req.URL = target
			req.Header.Set("X-Forwarded-Host", originalHost)

			// Forward the authority header which is important for HTTP/2 and gRPC
			// In HTTP/2, :authority should contain the host and optionally the port
			// It's equivalent to the Host header in HTTP/1.1
			if req.Header.Get(":authority") != "" {
				// Use originalHost which should already be in the correct format (host:port if non-default port)
				req.Header.Set(":authority", originalHost)
			}
			// Don't override the host if we want to preserve it
			// This ensures the target service sees the original host
			if !hostOverride {
				req.Host = originalHost
			}
		},
	}
}

func (h *Handler) GetQueueStatus(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	service := r.URL.Query().Get("service")

	queueSize := h.throttler.GetQueueSize(namespace, service)
	response := QueueStatusResponse{}

	if queueSize > 0 {
		response.QueueStatus = 1
	} else {
		response.QueueStatus = 0
	}

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		h.logger.Error("Failed to encode queue size response",
			zap.Error(err),
			zap.String("namespace", namespace),
			zap.String("service", service),
		)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
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

package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"

	"go.uber.org/zap"
)

type Handler struct {
	logger     *zap.Logger
	throttler  Throttler
	transport  http.RoundTripper
	targetURL  *url.URL
	bufferPool httputil.BufferPool
}

func NewHandler(ctx context.Context, logger *zap.Logger, transport http.RoundTripper, throttle Throttler, targetURLStr string) *Handler {
	targetURL, _ := url.Parse(targetURLStr)
	return &Handler{
		throttler:  throttle,
		logger:     logger,
		transport:  transport,
		targetURL:  targetURL,
		bufferPool: NewBufferPool(),
	}
}

type Response struct {
	Message string `json:"message"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx := context.Background()
	h.logger.Debug("Sending request for try")
	if tryErr := h.throttler.Try(ctx, func() error {
		// If the try is successful, how do we want to handle the reuqest.
		h.logger.Debug("Try successful, processing request")
		h.logger.Debug("Proxy Request",
			zap.Any("url", h.targetURL),
			zap.Any("header", req.Header),
			zap.Any("method", req.Method),
			zap.Any("proto", req.Proto),
			zap.Any("Req", req),
		)

		target := &url.URL{}
		if req.Host == "external-target-service.default.svc.cluster.local:8012" {
			// We can do the routing here based on the host, or we can use CRDs to do it
			target = req.
		}

		h.ProxyRequest(w, req, target)
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

func (h *Handler) ProxyRequest(w http.ResponseWriter, req *http.Request, targetURL *url.URL) {
	h.logger.Debug("Requesting Proxy")
	proxy := httputil.NewSingleHostReverseProxy(h.targetURL)
	proxy.BufferPool = h.bufferPool
	proxy.Transport = h.transport
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		h.logger.Info("proxy error handler triggered", zap.Error(err))
	}
	proxy.ServeHTTP(w, req)
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

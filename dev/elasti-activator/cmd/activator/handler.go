package main

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

	"go.uber.org/zap"
)

type Handler struct {
	logger     *zap.Logger
	throttler  Throttler
	transport  http.RoundTripper
	bufferPool httputil.BufferPool
	timeout    time.Duration
}

func NewHandler(ctx context.Context, logger *zap.Logger, transport http.RoundTripper, throttle Throttler) *Handler {
	return &Handler{
		throttler:  throttle,
		logger:     logger,
		transport:  transport,
		bufferPool: NewBufferPool(),
		timeout:    10 * time.Second,
	}
}

type Response struct {
	Message string `json:"message"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()
	host, err := HostManager.GetHost(req)
	if err != nil {
		h.logger.Error("Error getting host", zap.Error(err))
		http.Error(w, "Error getting host", http.StatusInternalServerError)
		return
	}
	h.logger.Debug("Host", zap.Any("host", host))
	if !host.TrafficAllowed {
		h.logger.Error("Traffic not allowed", zap.Any("host", host))
		w.Header().Set("Connection", "close")
		w.Write([]byte("Traffic is switched"))
		return
	}
	Informer.Inform(host.Namespace, host.SourceService)
	targetURL, err := url.Parse(host.TargetHost + req.RequestURI)
	if err != nil {
		h.logger.Error("Error parsing target URL", zap.Error(err))
		http.Error(w, "Error parsing target URL", http.StatusInternalServerError)
		return
	}
	select {
	case <-ctx.Done():
		h.logger.Error("Request timeout", zap.Error(ctx.Err()))
		w.WriteHeader(http.StatusInternalServerError)
	default:
		if tryErr := h.throttler.Try(ctx, host, func(count int) error {
			return h.ProxyRequest(w, req, targetURL, count)
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

func (h *Handler) ProxyRequest(w http.ResponseWriter, req *http.Request, targetURL *url.URL, count int) (rErr error) {
	defer func() {
		if r := recover(); r != nil {
			h.logger.Error("Recovered from panic", zap.Any("panic", r))
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			h.logger.Error("Panic stack trace", zap.ByteString("stacktrace", buf[:n]))
			rErr = r.(error)
		}
	}()
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.BufferPool = h.bufferPool
	proxy.Transport = h.transport
	req.Header.Set("elasti-retry-count", strconv.Itoa(count))
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		req.Header.Set("error", err.Error())
	}
	proxy.ServeHTTP(w, req)
	if err := w.Header().Get("error"); err != "" {
		h.logger.Error("error header found", zap.String("error", err))
		return errors.New(err)
	}
	h.logger.Debug("Proxy request completed")
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

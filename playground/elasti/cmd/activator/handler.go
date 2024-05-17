package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type Handler struct {
	logger     *zap.Logger
	throttler  Throttler
	transport  http.RoundTripper
	bufferPool httputil.BufferPool
}

func NewHandler(ctx context.Context, logger *zap.Logger, transport http.RoundTripper, throttle Throttler) *Handler {
	return &Handler{
		throttler:  throttle,
		logger:     logger,
		transport:  transport,
		bufferPool: NewBufferPool(),
	}
}

type Response struct {
	Message string `json:"message"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	h.logger.Debug("Proxy Request",
		zap.Any("host", req.Host),
		zap.Any("header", req.Header),
		zap.Any("method", req.Method),
		zap.Any("proto", req.Proto),
		zap.Any("Req URI", req.RequestURI),
		zap.Any("Req Body", req.Body),
		zap.Any("Req Remote Addr", req.RemoteAddr),
		zap.Any("Req URL", req.URL),
	)

	ns, svc := "empty", "empty"
	var targetHost string
	if values, ok := req.Header["X-Envoy-Decorator-Operation"]; ok {
		// Request coming from istio
		h.logger.Debug("X-Envoy-Decorator-Operation", zap.Any("values", values))
		targetHost = values[0]
		ns, svc, _ = h.extractNamespaceAndService(targetHost, false)
		svc += "-pvt"
	} else {
		// Request is coming from internal pods
		h.logger.Debug("Request no from istio")
		targetHost = req.Host
		ns, svc, _ = h.extractNamespaceAndService(req.Host, true)
		svc += "-pvt"
	}
	Informer.Inform(ns, svc)
	targetHost = replaceServiceName(targetHost, svc)
	targetHost = addHTTPIfNeeded(targetHost)
	targetHost = removeTrailingWildcardIfNeeded(targetHost)
	// TODOs: Handle this error
	targetURL, _ := url.Parse(targetHost + req.RequestURI)
	h.logger.Debug("Extracted Info",
		zap.String("ns", ns),
		zap.Any("svc", svc),
		zap.Any("target", targetURL),
		zap.Any("Host", targetHost))

	go func() {
		if tryErr := h.throttler.Try(ctx, ns, svc, func() error {
			h.ProxyRequest(w, req, targetURL)
			return nil
		}); tryErr != nil {
			h.logger.Error("throttler try error: ", zap.Error(tryErr))
			if errors.Is(tryErr, context.DeadlineExceeded) {
				http.Error(w, tryErr.Error(), http.StatusServiceUnavailable)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}
	}()

	// Timeout the request if it takes too long
	<-ctx.Done()
	h.logger.Error("Request timeout", zap.Error(ctx.Err()))
}

func (h *Handler) ProxyRequest(w http.ResponseWriter, req *http.Request, targetURL *url.URL) {
	h.logger.Debug("Requesting Proxy", zap.Any("targetURL", targetURL))
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
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

func addHTTPIfNeeded(serviceURL string) string {
	if !strings.HasPrefix(serviceURL, "http://") && !strings.HasPrefix(serviceURL, "https://") {
		return "http://" + serviceURL
	}
	return serviceURL
}

func removeTrailingWildcardIfNeeded(serviceURL string) string {
	if strings.HasSuffix(serviceURL, "/*") {
		return strings.TrimSuffix(serviceURL, "/*")
	}
	return serviceURL
}

func replaceServiceName(serviceURL, newServiceName string) string {
	parts := strings.Split(serviceURL, ".")
	if len(parts) < 3 {
		return serviceURL
	}
	parts[0] = newServiceName
	return strings.Join(parts, ".")
}

func (h *Handler) extractNamespaceAndService(s string, internal bool) (string, string, error) {
	re := regexp.MustCompile(`(?P<service>[^.]+)\.(?P<namespace>[^.]+)\.svc\.cluster\.local:\d+/\*`)
	// When the request come internal source, we don't get a http
	if internal {
		re = regexp.MustCompile(`(?P<service>[^.]+)\.(?P<namespace>[^.]+)\.svc\.cluster\.local:\d+`)
	}
	matches := re.FindStringSubmatch(s)
	if len(matches) < 3 {
		return "", "", fmt.Errorf("unable to extract namespace and service name")
	}
	service := matches[re.SubexpIndex("service")]
	namespace := matches[re.SubexpIndex("namespace")]
	return namespace, service, nil
}

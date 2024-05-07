package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"

	"go.uber.org/zap"
)

type Handler struct {
	logger    *zap.Logger
	throttler Throttler
	transport http.RoundTripper
	targetURL *url.URL
}

func NewHandler(ctx context.Context, logger *zap.Logger, transport http.RoundTripper, throttle Throttler, targetURLStr string) *Handler {
	targetURL, _ := url.Parse(targetURLStr)
	return &Handler{
		throttler: throttle,
		logger:    logger,
		transport: transport,
		targetURL: targetURL,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	h.logger.Debug("Sending request for try")
	if tryErr := h.throttler.Try(ctx, func(dest string) error {
		// If the try is successful, how do we want to handle the reuqest.
		h.logger.Debug("Try successful, processing request")
		h.ProxyRequest(w, r)
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

func (h *Handler) ProxyRequest(w http.ResponseWriter, req *http.Request) {
	h.logger.Debug("Requesting Proxy")

	proxy := httputil.NewSingleHostReverseProxy(h.targetURL)
	//proxy := h.NewHeaderPruningReverseProxy(h.targetURL.Host, "Proxy", false)
	proxy.Transport = h.transport
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		h.logger.Info("proxy error handler triggered", zap.Error(err))
	}

	req.Host = h.targetURL.Host
	h.logger.Debug("Proxy Request",
		zap.Any("url", h.targetURL),
		zap.Any("header", req.Header),
		zap.Any("method", req.Method),
		zap.Any("proto", req.Proto),
		zap.Any("host", req.Host),
	)

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

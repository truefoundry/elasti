package main

import (
	"context"
	"net/http"

	"go.uber.org/zap"
)

func main() {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()

	PodIP := "192.168.0.1"
	MaxIdleProxyConns := 100
	MaxIdleProxyConnsPerHost := 1000

	logger.Info("fields", zap.Int("MaxIdleProxyConns", MaxIdleProxyConns), zap.Int("MaxIdleProxyConnsPerHost", MaxIdleProxyConnsPerHost))

	transport := NewProxyAutoTransport(MaxIdleProxyConns, MaxIdleProxyConnsPerHost)
	logger.Debug("Transport initiated")
	throttler := NewThrottler(ctx, logger, PodIP)
	logger.Debug("Throttler initiated")
	handler := NewHandler(ctx, logger, transport, *throttler)
	logger.Debug("Handler initiated")

	http.HandleFunc("/", handler.ServeHTTP)

	logger.Info("Reverse Proxy Server starting at ", zap.String("port", "8000"))
	if err := http.ListenAndServe(":8000", nil); err != nil {
		logger.Fatal("ListenAndServe Failed: ", zap.Error(err))
	}
}

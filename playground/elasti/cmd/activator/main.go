package main

import (
	"context"
	"log"
	"net/http"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
)

type config struct {
	PodName                  string `split_words:"true" required:"true"`
	PodIP                    string `split_words:"true" required:"true"`
	MaxIdleProxyConns        int    `split_words:"true" default:"1000"`
	MaxIdleProxyConnsPerHost int    `split_words:"true" default:"100"`
}

func main() {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()

	var env config
	if err := envconfig.Process("", &env); err != nil {
		log.Fatal("Failed to process env: ", err)
	}

	logger.Info("config", zap.String("PodName", env.PodName), zap.String("PodIP", env.PodIP), zap.Int("MaxIdleProxyConns", env.MaxIdleProxyConns), zap.Int("MaxIdleProxyConnsPerHost", env.MaxIdleProxyConnsPerHost))

	transport := NewProxyAutoTransport(logger, env.MaxIdleProxyConns, env.MaxIdleProxyConnsPerHost)
	logger.Debug("Transport initiated")
	throttler := NewThrottler(ctx, logger, env.PodIP)
	logger.Debug("Throttler initiated")
	handler := NewHandler(ctx, logger, transport, *throttler, env.PodIP)
	logger.Debug("Handler initiated")

	http.HandleFunc("/", handler.ServeHTTP)

	logger.Info("Reverse Proxy Server starting at ", zap.String("port", "8012"))
	if err := http.ListenAndServe(":8012", nil); err != nil {
		logger.Fatal("ListenAndServe Failed: ", zap.Error(err))
	}
}

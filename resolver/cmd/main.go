package main

import (
	"context"
	"log"
	"net/http"
	"time"
	"truefoundry/resolver/internal/handler"
	"truefoundry/resolver/internal/hostManager"
	"truefoundry/resolver/internal/operator"
	"truefoundry/resolver/internal/utils"

	"github.com/kelseyhightower/envconfig"
	"github.com/truefoundry/elasti/pkg/logger"
	"go.uber.org/zap"
)

type config struct {
	MaxIdleProxyConns        int `split_words:"true" default:"1000"`
	MaxIdleProxyConnsPerHost int `split_words:"true" default:"100"`
}

const (
	port                    = ":8012"
	trafficReenableDuration = 30 * time.Second
	operatorRetryDuration   = 30 * time.Second
)

func main() {
	ctx := context.Background()
	logger, err := logger.GetLogger("dev")
	if err != nil {
		log.Fatal("Failed to get logger: ", err)
	}
	var env config
	if err := envconfig.Process("", &env); err != nil {
		log.Fatal("Failed to process env: ", err)
	}
	logger.Info("config", zap.Int("MaxIdleProxyConns", env.MaxIdleProxyConns), zap.Int("MaxIdleProxyConnsPerHost", env.MaxIdleProxyConnsPerHost))
	operatorRPC := operator.NewOperatorClient(logger, operatorRetryDuration)
	hostManager := hostManager.NewHostManager(logger, trafficReenableDuration)
	k8sUtil := utils.NewK8sUtil(logger)
	handler := handler.NewHandler(ctx, logger, &handler.HandlerConfig{
		MaxIdleProxyConns:        env.MaxIdleProxyConns,
		MaxIdleProxyConnsPerHost: env.MaxIdleProxyConnsPerHost,
		OperatorRPC:              operatorRPC,
		HostManager:              hostManager,
		K8sUtil:                  k8sUtil,
	})
	http.HandleFunc("/", handler.ServeHTTP)
	logger.Info("Reverse Proxy Server starting at ", zap.String("port", port))
	if err := http.ListenAndServe(port, nil); err != nil {
		logger.Fatal("ListenAndServe Failed: ", zap.Error(err))
	}
}

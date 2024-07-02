package main

import (
	"context"
	"log"
	"net/http"
	"time"
	"truefoundry/resolver/internal/handler"
	"truefoundry/resolver/internal/hostManager"
	"truefoundry/resolver/internal/operator"

	"github.com/kelseyhightower/envconfig"
	"github.com/truefoundry/elasti/pkg/k8sHelper"
	"github.com/truefoundry/elasti/pkg/logger"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
)

type config struct {
	MaxIdleProxyConns        int `split_words:"true" default:"1000"`
	MaxIdleProxyConnsPerHost int `split_words:"true" default:"100"`
}

const (
	port                    = ":8012"
	trafficReEnableDuration = 30 * time.Second
	operatorRetryDuration   = 30 * time.Second
)

func main() {
	ctx := context.Background()
	logger, err := logger.NewLogger("dev")
	if err != nil {
		log.Fatal("Failed to get logger: ", err)
	}

	// Read env values
	var env config
	if err := envconfig.Process("", &env); err != nil {
		log.Fatal("Failed to process env: ", err)
	}
	logger.Info("config", zap.Int("MaxIdleProxyConns", env.MaxIdleProxyConns), zap.Int("MaxIdleProxyConnsPerHost", env.MaxIdleProxyConnsPerHost))

	// Get kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		logger.Fatal("Error fetching cluster config", zap.Error(err))
	}

	k8sUtil := k8sHelper.NewOps(logger, config)
	operatorRPC := operator.NewOperatorClient(logger, operatorRetryDuration)
	reqHostManager := hostManager.NewHostManager(logger, trafficReEnableDuration)
	requestHandler := handler.NewHandler(ctx, logger, &handler.HandlerConfig{
		MaxIdleProxyConns:        env.MaxIdleProxyConns,
		MaxIdleProxyConnsPerHost: env.MaxIdleProxyConnsPerHost,
		OperatorRPC:              operatorRPC,
		HostManager:              reqHostManager,
		K8sUtil:                  k8sUtil,
	})

	// Handle all the incoming requests
	http.HandleFunc("/", requestHandler.ServeHTTP)
	logger.Info("Reverse Proxy Server starting at ", zap.String("port", port))
	if err := http.ListenAndServe(port, nil); err != nil {
		logger.Fatal("ListenAndServe Failed: ", zap.Error(err))
	}
}

package main

import (
	"log"
	"net/http"
	"time"
	"truefoundry/elasti/resolver/internal/handler"
	"truefoundry/elasti/resolver/internal/hostManager"
	"truefoundry/elasti/resolver/internal/operator"
	"truefoundry/elasti/resolver/internal/throttler"

	"github.com/getsentry/sentry-go"
	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/truefoundry/elasti/pkg/k8sHelper"
	"github.com/truefoundry/elasti/pkg/logger"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
	"github.com/truefoundry/elasti/pkg/utils"
)

type config struct {
	MaxIdleProxyConns        int `split_words:"true" default:"1000"`
	MaxIdleProxyConnsPerHost int `split_words:"true" default:"100"`
	// ReqTimeout is the timeout for each request
	ReqTimeout int `split_words:"true" default:"10"`
	// TrafficReEnableDuration is the duration for which the traffic is disabled for a host
	// This is also duration for which we don't recheck readiness of the service
	TrafficReEnableDuration int `split_words:"true" default:"30"`
	// OperatorRetryDuration is the duration for which we don't inform the operator
	// about the traffic on the same host
	OperatorRetryDuration int `split_words:"true" default:"30"`
	// QueueRetryDuration is the duration after we retry the requests in queue
	QueueRetryDuration int `split_words:"true" default:"5"`
	// QueueSize is the size of the queue
	QueueSize int `split_words:"true" default:"100"`
	// MaxQueueConcurrency is the maximum number of concurrent requests
	MaxQueueConcurrency int `split_words:"true" default:"10"`
	// InitialCapacity is the initial capacity of the semaphore
	InitialCapacity int `split_words:"true" default:"100"`
	// HeaderForHost is the header to look for to get the host
	HeaderForHost string `split_words:"true" default:"Host"`
	// AUTH_SERVER_URL is the URL of the auth server
	AuthServerURL string `split_words:"true" required:"true"`
	// TENANT_NAME is the tenant name
	TenantName string `split_words:"true"`
}

const (
	port = ":8012"
	flushTimeout 		= 2 * time.Second
)

func main() {
	logger, err := logger.NewLogger("dev")
	if err != nil {
		log.Fatal("Failed to get logger: ", err)
	}
	var env config
	if err := envconfig.Process("", &env); err != nil {
		log.Fatal("Failed to process env: ", err)
	}
	logger.Info("config", zap.Any("env", env))
	config, err := rest.InClusterConfig()
	if err != nil {
		logger.Fatal("Error fetching cluster config", zap.Error(err))
	}

	authData, err := utils.GetSentryAuthData(env.AuthServerURL, env.TenantName)
	if err != nil {
		logger.Error("Error fetching sentry auth data", zap.Error(err))
	} else {
		utils.InitializeSentry(logger, authData, "resolver", env.TenantName)
		defer sentry.Flush(flushTimeout)
	}

	// Get components required for the handler
	k8sUtil := k8sHelper.NewOps(logger, config)
	newOperatorRPC := operator.NewOperatorClient(logger, time.Duration(env.OperatorRetryDuration)*time.Second)
	newHostManager := hostManager.NewHostManager(logger, time.Duration(env.TrafficReEnableDuration)*time.Second, env.HeaderForHost)
	newTransport := throttler.NewProxyAutoTransport(logger, env.MaxIdleProxyConns, env.MaxIdleProxyConnsPerHost)
	newThrottler := throttler.NewThrottler(&throttler.ThrottlerParams{
		QueueRetryDuration:      time.Duration(env.QueueRetryDuration) * time.Second,
		K8sUtil:                 k8sUtil,
		QueueDepth:              env.QueueSize,
		MaxConcurrency:          env.MaxQueueConcurrency,
		InitialCapacity:         env.InitialCapacity,
		TrafficReEnableDuration: time.Duration(env.TrafficReEnableDuration) * time.Second,
		Logger:                  logger,
	})

	// Create a handler
	requestHandler := handler.NewHandler(&handler.HandlerParams{
		Logger:      logger,
		ReqTimeout:  time.Duration(env.ReqTimeout) * time.Second,
		OperatorRPC: newOperatorRPC,
		HostManager: newHostManager,
		Throttler:   newThrottler,
		Transport:   newTransport,
	})

	// Handle all the incoming requests
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", requestHandler.ServeHTTP)
	logger.Info("Reverse Proxy Server starting at ", zap.String("port", port))
	if err := http.ListenAndServe(port, nil); err != nil {
		logger.Fatal("ListenAndServe Failed: ", zap.Error(err))
	}
}
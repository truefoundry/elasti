package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"

	sentryhttp "github.com/getsentry/sentry-go/http"

	"github.com/truefoundry/elasti/resolver/internal/handler"
	"github.com/truefoundry/elasti/resolver/internal/hostmanager"
	"github.com/truefoundry/elasti/resolver/internal/operator"
	"github.com/truefoundry/elasti/resolver/internal/throttler"

	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/truefoundry/elasti/pkg/k8shelper"
	"github.com/truefoundry/elasti/pkg/logger"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
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
	// Sentry config
	SentryDsn string `split_words:"true" default:""`
	SentryEnv string `envconfig:"SENTRY_ENVIRONMENT" default:""`
}

const (
	reverseProxyPort = ":8012"
	internalPort     = ":8013"
)

func main() {
	var env config
	if err := envconfig.Process("", &env); err != nil {
		log.Fatal("Failed to process env: ", err)
	}

	sentryEnabled := env.SentryDsn != ""

	if sentryEnabled {
		fmt.Println("Initializing Sentry")
		if err := sentry.Init(sentry.ClientOptions{
			Dsn:              env.SentryDsn,
			EnableTracing:    false,
			TracesSampleRate: 1.0,
			Environment:      env.SentryEnv,
		}); err != nil {
			fmt.Println("Sentry initialization failed:", err)
		}
		defer sentry.Flush(2 * time.Second)
	}

	logger, err := logger.NewLogger("dev", sentryEnabled)
	if err != nil {
		log.Fatal("Failed to get logger: ", err)
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		logger.Fatal("Error fetching cluster config", zap.Error(err))
	}

	// Get components required for the handler
	k8sUtil := k8shelper.NewOps(logger, config)
	newOperatorRPC := operator.NewOperatorClient(logger, time.Duration(env.OperatorRetryDuration)*time.Second)
	newHostManager := hostmanager.NewHostManager(logger, time.Duration(env.TrafficReEnableDuration)*time.Second, env.HeaderForHost)
	newTransport := throttler.NewProxyAutoTransport(env.MaxIdleProxyConns, env.MaxIdleProxyConnsPerHost)
	newThrottler := throttler.NewThrottler(&throttler.Params{
		QueueRetryDuration:      time.Duration(env.QueueRetryDuration) * time.Second,
		K8sUtil:                 k8sUtil,
		QueueDepth:              env.QueueSize,
		MaxConcurrency:          env.MaxQueueConcurrency,
		InitialCapacity:         env.InitialCapacity,
		TrafficReEnableDuration: time.Duration(env.TrafficReEnableDuration) * time.Second,
		Logger:                  logger,
	})

	// Create an instance of sentryhttp
	sentryHandler := sentryhttp.New(sentryhttp.Options{})

	// Create a handler
	requestHandler := handler.NewHandler(&handler.Params{
		Logger:      logger,
		ReqTimeout:  time.Duration(env.ReqTimeout) * time.Second,
		OperatorRPC: newOperatorRPC,
		HostManager: newHostManager,
		Throttler:   newThrottler,
		Transport:   newTransport,
	})

	// Handle all the incoming requests
	reverseProxyServerMux := http.NewServeMux()
	reverseProxyServerMux.Handle("/", sentryHandler.HandleFunc(requestHandler.ServeHTTP))
	reverseProxyServer := &http.Server{
		Addr:              reverseProxyPort,
		Handler:           reverseProxyServerMux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	logger.Info("Reverse Proxy Server starting at ", zap.String("port", reverseProxyPort))
	go func() {
		if err := reverseProxyServer.ListenAndServe(); err != nil {
			logger.Fatal("ListenAndServe Failed: ", zap.Error(err))
		}
	}()

	// Handle all the incoming internal request like from prometheus that are not related to the reverse proxy
	internalServeMux := http.NewServeMux()
	internalServeMux.Handle("/metrics", promhttp.Handler())
	internalServeMux.Handle("/queue-status", sentryHandler.HandleFunc(requestHandler.GetQueueStatus))
	internalServer := &http.Server{
		Addr:              internalPort,
		Handler:           internalServeMux,
		ReadHeaderTimeout: 2 * time.Second,
	}
	logger.Info("Internal Server starting at ", zap.String("port", internalPort))
	if err := internalServer.ListenAndServe(); err != nil {
		logger.Fatal("ListenAndServe Failed: ", zap.Error(err))
	}
}

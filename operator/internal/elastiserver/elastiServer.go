package elastiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	sentryhttp "github.com/getsentry/sentry-go/http"

	"k8s.io/client-go/rest"

	"truefoundry/elasti/operator/internal/crddirectory"
	"truefoundry/elasti/operator/internal/prom"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/truefoundry/elasti/pkg/k8shelper"
	"github.com/truefoundry/elasti/pkg/messages"
	"go.uber.org/zap"
)

type (
	Response struct {
		Message string `json:"message"`
	}

	// Server is used to receive communication from Resolver, or any future components
	// It is used by components about certain events, like when resolver receive the request
	// for a service, that service is scaled up if it's at 0 replicas
	Server struct {
		logger     *zap.Logger
		k8shelper  *k8shelper.Ops
		scaleLocks sync.Map
		// rescaleDuration is the duration to wait before checking to rescaling the target
		rescaleDuration time.Duration
	}
)

func NewServer(logger *zap.Logger, config *rest.Config, rescaleDuration time.Duration) *Server {
	// Get Ops client
	k8sUtil := k8shelper.NewOps(logger, config)
	return &Server{
		logger:          logger.Named("elastiServer"),
		k8shelper:       k8sUtil,
		rescaleDuration: rescaleDuration,
	}
}

// Start starts the ElastiServer and declares the endpoint and handlers for it
func (s *Server) Start(port string) error {
	mux := http.NewServeMux()
	sentryHandler := sentryhttp.New(sentryhttp.Options{})
	mux.Handle("/metrics", sentryHandler.Handle(promhttp.Handler()))
	mux.Handle("/informer/incoming-request", sentryHandler.HandleFunc(s.resolverReqHandler))

	server := &http.Server{
		Addr:              fmt.Sprintf(":%s", strings.TrimPrefix(port, ":")),
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}

	// Graceful shutdown handling
	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		s.logger.Info("Server is shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			s.logger.Error("Could not gracefully shutdown the server", zap.Error(err))
		}
		close(done)
	}()

	s.logger.Info("Starting ElastiServer", zap.String("port", port))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.logger.Error("Failed to start ElastiServer", zap.Error(err))
		return err
	}

	<-done
	s.logger.Info("Server stopped")
	return nil
}

func (s *Server) resolverReqHandler(w http.ResponseWriter, req *http.Request) {
	ctx := context.Background()
	if req.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	var body messages.RequestCount
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	defer func(b io.ReadCloser) {
		err := b.Close()
		if err != nil {
			s.logger.Error("Failed to close Body", zap.Error(err))
		}
	}(req.Body)
	s.logger.Info("-- Received request from Resolver", zap.Any("body", body))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := Response{
		Message: "Request received successfully!",
	}
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = w.Write(jsonResponse)
	if err != nil {
		s.logger.Error("Failed to write response", zap.Error(err))
		return
	}
	err = s.scaleTargetForService(ctx, body.Svc, body.Namespace)
	if err != nil {
		s.logger.Error("Failed to compare and scale target", zap.Error(err))
		return
	}
	s.logger.Info("-- Received fulfilled from Resolver", zap.Any("body", body))
}

func (s *Server) scaleTargetForService(_ context.Context, serviceName, namespace string) error {
	scaleMutex, loaded := s.getMutexForServiceScale(serviceName)
	if loaded {
		return nil
	}
	scaleMutex.Lock()
	defer s.logger.Debug("Scale target lock released")
	s.logger.Debug("Scale target lock taken")
	crd, found := crddirectory.CRDDirectory.GetCRD(serviceName)
	if !found {
		s.releaseMutexForServiceScale(serviceName)
		return fmt.Errorf("scaleTargetForService - error: failed to get CRD details from directory, serviceName: %s", serviceName)
	}

	if err := s.k8shelper.ScaleTargetWhenAtZero(namespace, crd.Spec.ScaleTargetRef.Name, crd.Spec.ScaleTargetRef.Kind, crd.Spec.MinTargetReplicas); err != nil {
		s.releaseMutexForServiceScale(serviceName)
		prom.TargetScaleCounter.WithLabelValues(serviceName, crd.Spec.ScaleTargetRef.Kind+"-"+crd.Spec.ScaleTargetRef.Name, err.Error()).Inc()
		return fmt.Errorf("scaleTargetForService - error: %w, targetRefKind: %s, targetRefName: %s", err, crd.Spec.ScaleTargetRef.Kind, crd.Spec.ScaleTargetRef.Name)
	}
	prom.TargetScaleCounter.WithLabelValues(serviceName, crd.Spec.ScaleTargetRef.Kind+"-"+crd.Spec.ScaleTargetRef.Name, "success").Inc()

	// If the target is scaled up, we will hold the lock for longer, to not scale up again
	time.AfterFunc(s.rescaleDuration, func() {
		s.releaseMutexForServiceScale(serviceName)
	})
	return nil
}

func (s *Server) releaseMutexForServiceScale(service string) {
	lock, loaded := s.scaleLocks.Load(service)
	if !loaded {
		return
	}
	lock.(*sync.Mutex).Unlock()
	s.scaleLocks.Delete(service)
}

func (s *Server) getMutexForServiceScale(serviceName string) (*sync.Mutex, bool) {
	l, loaded := s.scaleLocks.LoadOrStore(serviceName, &sync.Mutex{})
	return l.(*sync.Mutex), loaded
}

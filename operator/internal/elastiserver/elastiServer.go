package elastiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	sentryhttp "github.com/getsentry/sentry-go/http"
	"k8s.io/apimachinery/pkg/types"

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
		logger:    logger.Named("elastiServer"),
		k8shelper: k8sUtil,
		// rescaleDuration is the duration to wait before checking to rescaling the target
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
	defer func() {
		if err := req.Body.Close(); err != nil {
			s.logger.Error("Failed to close request body", zap.Error(err))
		}
	}()

	if req.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	var body messages.RequestCount
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		s.logger.Error("Failed to decode request body", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	s.logger.Info("Received request from Resolver", zap.Any("body", body))

	response := Response{
		Message: "Request received successfully!",
	}
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		s.logger.Error("Failed to marshal response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if _, err = w.Write(jsonResponse); err != nil {
		s.logger.Error("Failed to write response", zap.Error(err))
		return
	}

	if err = s.scaleTargetForService(req.Context(), body.Svc, body.Namespace); err != nil {
		s.logger.Error("Failed to scale target",
			zap.Error(err),
			zap.String("service", body.Svc),
			zap.String("namespace", body.Namespace))
		return
	}

	s.logger.Info("Request fulfilled successfully",
		zap.String("service", body.Svc),
		zap.String("namespace", body.Namespace))
}

func (s *Server) scaleTargetForService(_ context.Context, serviceName, namespace string) error {
	namespacedName := (types.NamespacedName{Namespace: namespace, Name: serviceName}).String()
	scaleMutex, loaded := s.getMutexForServiceScale(namespacedName)
	if loaded {
		s.logger.Debug("Scale target lock already exists", zap.String("service", namespacedName))
		return nil
	}
	scaleMutex.Lock()

	defer s.logger.Debug("Scale target lock released", zap.String("service", namespacedName))
	s.logger.Debug("Scale target lock taken", zap.String("service", namespacedName))

	crd, found := crddirectory.CRDDirectory.GetCRD(namespacedName)
	if !found {
		s.releaseMutexForServiceScale(namespacedName)
		return fmt.Errorf("scaleTargetForService - error: failed to get CRD details from directory, namespacedName: %s", namespacedName)
	}

	if err := s.k8shelper.ScaleTargetWhenAtZero(namespace, crd.Spec.ScaleTargetRef.Name, crd.Spec.ScaleTargetRef.Kind, crd.Spec.MinTargetReplicas); err != nil {
		s.releaseMutexForServiceScale(namespacedName)
		prom.TargetScaleCounter.WithLabelValues(serviceName, namespace, crd.Spec.ScaleTargetRef.Kind+"-"+crd.Spec.ScaleTargetRef.Name, err.Error()).Inc()
		return fmt.Errorf("scaleTargetForService - error: %w, targetRefKind: %s, targetRefName: %s", err, crd.Spec.ScaleTargetRef.Kind, crd.Spec.ScaleTargetRef.Name)
	}
	prom.TargetScaleCounter.WithLabelValues(serviceName, namespace, crd.Spec.ScaleTargetRef.Kind+"-"+crd.Spec.ScaleTargetRef.Name, "success").Inc()

	// If the target is scaled up, we will hold the lock for longer, to not scale up again
	// TODO: Is there a better way to do this and why is it even needed?
	time.AfterFunc(s.rescaleDuration, func() {
		s.releaseMutexForServiceScale(namespacedName)
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

package elastiServer

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/truefoundry/elasti/pkg/k8sHelper"
	"github.com/truefoundry/elasti/pkg/messages"
	"go.uber.org/zap"
	"truefoundry.io/elasti/internal/crdDirectory"
)

type (
	Response struct {
		Message string `json:"message"`
	}

	// Server is used to receive communication from Resolver, or any future components
	// It is used by components about certain events, like when resolver receive the request
	// for a service, that service is scaled up if it's at 0 replicas
	Server struct {
		logger    *zap.Logger
		k8sHelper *k8sHelper.Ops
	}
)

func NewServer(logger *zap.Logger, config *rest.Config) *Server {
	// Get kubernetes client
	kClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Fatal("Error connecting with kubernetes", zap.Error(err))
	}
	// Get Ops client
	k8sUtil := k8sHelper.NewOps(logger, kClient)
	return &Server{
		logger:    logger.Named("elastiServer"),
		k8sHelper: k8sUtil,
	}
}

// Start starts the ElastiServer and declares the endpoint and handlers for it
func (s *Server) Start(port string) {
	defer func() {
		if rec := recover(); s != nil {
			s.logger.Error("ElastiServer is recovering from panic", zap.Any("error", rec))
			go s.Start(port)
		}
	}()
	http.HandleFunc("/informer/incoming-request", s.resolverReqHandler)
	s.logger.Info("Starting ElastiServer", zap.String("port", port))
	if err := http.ListenAndServe(port, nil); err != nil {
		s.logger.Fatal("Failed to start StartElastiServer", zap.Error(err))
	}
}

func (s *Server) resolverReqHandler(w http.ResponseWriter, req *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			s.logger.Error("Recovered from panic", zap.Any("error", rec))
		}
	}()
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
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			s.logger.Error("Failed to close Body", zap.Error(err))
		}
	}(req.Body)
	s.logger.Info("Received request from Resolver", zap.Any("body", body))
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
		s.logger.Error("Failed to compare and scale deployment", zap.Error(err))
		return
	}
	s.logger.Info("Received fulfilled from Resolver", zap.Any("body", body))
}

func (s *Server) scaleTargetForService(_ context.Context, serviceName, namespace string) error {
	crd, found := crdDirectory.CRDDirectory.GetCRD(serviceName)
	if !found {
		s.logger.Error("Failed to get CRD details from directory")
	}
	if err := s.k8sHelper.ScaleTargetWhenAtZero(namespace, crd.Spec.ScaleTargetRef.Name, crd.Spec.ScaleTargetRef.Kind, crd.Spec.MinTargetReplicas); err != nil {
		s.logger.Error("Failed to scale TargetRef", zap.Any("TargetRef", crd.Spec.ScaleTargetRef), zap.Error(err))
		return err
	}
	s.logger.Info("TargetRef is scaled up", zap.Any("TargetRef", crd.Spec.ScaleTargetRef))
	return nil
}

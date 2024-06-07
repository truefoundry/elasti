package elastiServer

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/truefoundry/elasti/pkg/messages"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"truefoundry.io/elasti/internal/crdDirectory"
)

type (
	Response struct {
		Message string `json:"message"`
	}

	Server struct {
		logger  *zap.Logger
		kClient *kubernetes.Clientset
	}
)

const (
	Port = ":8013"
)

func NewServer(logger *zap.Logger, kConfig *rest.Config) *Server {
	kClient, err := kubernetes.NewForConfig(kConfig)
	if err != nil {
		logger.Fatal("Error connecting with kubernetes", zap.Error(err))
	}
	return &Server{
		logger:  logger.Named("elastiServer"),
		kClient: kClient,
	}
}

func (s *Server) Start() {
	defer func() {
		if rec := recover(); s != nil {
			s.logger.Error("ElastiServer is recovering from panic", zap.Any("error", rec))
			go s.Start()
		}
	}()
	http.HandleFunc("/informer/incoming-request", s.resolverReqHandler)
	s.logger.Info("Starting ElastiServer", zap.String("port", Port))
	if err := http.ListenAndServe(Port, nil); err != nil {
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
	deployment, found := crdDirectory.CRDDirectory.GetCRD(body.Svc)
	if !found {
		s.logger.Error("Failed to get CRD details from directory", zap.Error(err))
	}
	namespace := types.NamespacedName{
		Name:      deployment.DeploymentName,
		Namespace: body.Namespace,
	}
	err = s.compareAndScaleDeployment(ctx, namespace)
	if err != nil {
		s.logger.Error("Failed to compare and scale deployment", zap.Error(err))
		return
	}
	s.logger.Info("Received fulfilled from Resolver", zap.Any("body", body))
}

func (s *Server) compareAndScaleDeployment(_ context.Context, _ types.NamespacedName) error {
	// deployment := &appsv1.Deployment{}
	// if err := s.Get(ctx, target, deployment); err != nil {
	// 	if errors.IsNotFound(err) {
	// 		s.logger.Info("Deployment not found", zap.Any("Deployment", target))
	// 		return nil
	// 	}
	// 	s.logger.Error("Failed to get Deployment", zap.Error(err))
	// 	return err
	// }
	// s.logger.Debug("Deployment found", zap.Any("Deployment", target))

	// // TODOs: This scaling might fail if some other process updates the deployment object
	// if *deployment.Spec.Replicas == 0 {
	// 	*deployment.Spec.Replicas = 1
	// 	if err := s.Update(ctx, deployment); err != nil {
	// 		s.logger.Error("Failed to scale up the Deployment", zap.Error(err))
	// 		return err
	// 	}
	// 	s.logger.Info("Deployment is scaled up", zap.Any("Deployment", target))
	// } else {
	// 	s.logger.Info("Deployment is already scaled up, nothing required from our end", zap.Any("Deployment", target))
	// }
	return nil
}

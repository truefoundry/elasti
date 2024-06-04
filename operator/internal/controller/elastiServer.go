package controller

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/truefoundry/elasti/pkg/messages"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

type Response struct {
	Message string `json:"message"`
}

func (r *ElastiServiceReconciler) StartElastiServer() {
	defer func() {
		if rec := recover(); r != nil {
			r.Logger.Error("ElastiServer is recovering from panic", zap.String("component", "elastiServer"), zap.Any("error", rec))
			go r.StartElastiServer()
		}
	}()
	http.HandleFunc("/informer/incoming-request", r.resolverReqHandler)
	r.Logger.Info("Starting ElastiServer", zap.String("port", "8013"))
	if err := http.ListenAndServe(":8013", nil); err != nil {
		r.Logger.Fatal("Failed to start StartElastiServer", zap.String("component", "elastiServer"), zap.Error(err))
	}
}

func (r *ElastiServiceReconciler) resolverReqHandler(w http.ResponseWriter, req *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			r.Logger.Error("Recovered from panic", zap.Any("error", rec))
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
	defer req.Body.Close()
	r.Logger.Info("Received request from Resolver", zap.String("component", "elastiServer"), zap.Any("body", body))
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
	w.Write(jsonResponse)
	// TODOs: The deployment name should be dynamic
	namespace := types.NamespacedName{
		Name:      "target",
		Namespace: body.Namespace,
	}
	r.compareAndScaleDeployment(ctx, namespace)
	r.Logger.Info("Received fullfilled from Resolver", zap.String("component", "elastiServer"), zap.Any("body", body))
}

func (r *ElastiServiceReconciler) compareAndScaleDeployment(ctx context.Context, target types.NamespacedName) error {
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, target, deployment); err != nil {
		if errors.IsNotFound(err) {
			r.Logger.Info("Deployment not found", zap.String("component", "elastiServer"), zap.Any("Deployment", target))
			return nil
		}
		r.Logger.Error("Failed to get Deployment", zap.String("component", "elastiServer"), zap.Error(err))
		return err
	}
	r.Logger.Debug("Deployment found", zap.String("component", "elastiServer"), zap.Any("Deployment", target))

	// TODOs: This scaling might fail if some other process updates the deployment object
	if *deployment.Spec.Replicas == 0 {
		*deployment.Spec.Replicas = 1
		if err := r.Update(ctx, deployment); err != nil {
			r.Logger.Error("Failed to scale up the Deployment", zap.String("component", "elastiServer"), zap.Error(err))
			return err
		}
		r.Logger.Info("Deployment is scaled up", zap.String("component", "elastiServer"), zap.Any("Deployment", target))
	} else {
		r.Logger.Info("Deployment is already scaled up, nothing required from our end", zap.String("component", "elastiServer"), zap.Any("Deployment", target))
	}
	return nil
}

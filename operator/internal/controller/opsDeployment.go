package controller

import (
	"context"
	"fmt"
	"strings"
	"truefoundry/elasti/operator/api/v1alpha1"
	"truefoundry/elasti/operator/internal/crddirectory"

	"github.com/truefoundry/elasti/pkg/k8shelper"
	"github.com/truefoundry/elasti/pkg/values"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *ElastiServiceReconciler) handleTargetDeploymentChanges(ctx context.Context, obj interface{}, _ *v1alpha1.ElastiService, req ctrl.Request) error {
	targetDeployment := &appsv1.Deployment{}
	err := k8shelper.UnstructuredToResource(obj, targetDeployment)
	if err != nil {
		return fmt.Errorf("failed to convert unstructured to deployment: %w", err)
	}
	if targetDeployment.Status.Replicas == 0 {
		r.Logger.Info("ScaleTargetRef Deployment has 0 replicas", zap.String("deployment_name", targetDeployment.Name), zap.String("es", req.String()))
		if err := r.switchMode(ctx, req, values.ProxyMode); err != nil {
			return fmt.Errorf("failed to switch mode: %w", err)
		}
	} else if targetDeployment.Status.ReadyReplicas > 0 {
		r.Logger.Info("ScaleTargetRef Deployment has ready replicas", zap.String("deployment_name", targetDeployment.Name), zap.String("es", req.String()))
		if err := r.switchMode(ctx, req, values.ServeMode); err != nil {
			return fmt.Errorf("failed to switch mode: %w", err)
		}
	}
	return nil
}

func (r *ElastiServiceReconciler) handleResolverChanges(ctx context.Context, obj interface{}) error {
	resolverDeployment := &appsv1.Deployment{}
	err := k8shelper.UnstructuredToResource(obj, resolverDeployment)
	if err != nil {
		return fmt.Errorf("failed to convert unstructured to deployment: %w", err)
	}
	if resolverDeployment.Name != resolverDeploymentName {
		return nil
	}

	crddirectory.CRDDirectory.Services.Range(func(key, value interface{}) bool {
		crdDetails := value.(*crddirectory.CRDDetails)
		if crdDetails.Status.Mode != values.ProxyMode {
			return true
		}

		// Extract namespace and service name from the key
		keyStr := key.(string)
		parts := strings.Split(keyStr, "/")
		if len(parts) != 2 {
			r.Logger.Error("Invalid key format", zap.String("key", keyStr))
			return true
		}
		namespacedName := types.NamespacedName{
			Namespace: parts[0],
			Name:      parts[1],
		}

		targetService := &v1.Service{}
		if err := r.Get(ctx, namespacedName, targetService); err != nil {
			r.Logger.Warn("Failed to get service to update EndpointSlice", zap.Error(err))
			return true
		}

		if err := r.createOrUpdateEndpointsliceToResolver(ctx, targetService); err != nil {
			r.Logger.Error("Failed to update EndpointSlice",
				zap.String("service", crdDetails.CRDName),
				zap.Error(err))
		}
		return true
	})

	return nil
}

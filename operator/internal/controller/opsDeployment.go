package controller

import (
	"context"
	"fmt"

	"truefoundry/elasti/operator/api/v1alpha1"

	"github.com/truefoundry/elasti/pkg/k8sHelper"
	"github.com/truefoundry/elasti/pkg/values"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *ElastiServiceReconciler) handleTargetDeploymentChanges(ctx context.Context, obj interface{}, _ *v1alpha1.ElastiService, req ctrl.Request) error {
	targetDeployment := &appsv1.Deployment{}
	err := k8sHelper.UnstructuredToResource(obj, targetDeployment)
	if err != nil {
		return fmt.Errorf("failed to convert unstructured to deployment: %w", err)
	}
	condition := targetDeployment.Status.Conditions
	if targetDeployment.Status.Replicas == 0 {
		r.Logger.Info("ScaleTargetRef Deployment has 0 replicas", zap.String("deployment_name", targetDeployment.Name), zap.String("es", req.String()))
		if _, err := r.switchMode(ctx, req, values.ProxyMode); err != nil {
			return fmt.Errorf("failed to switch mode: %w", err)
		}
	} else if targetDeployment.Status.Replicas > 0 && condition[1].Status == values.DeploymentConditionStatusTrue {
		r.Logger.Info("ScaleTargetRef Deployment has ready replicas", zap.String("deployment_name", targetDeployment.Name), zap.String("es", req.String()))
		if _, err := r.switchMode(ctx, req, values.ServeMode); err != nil {
			return fmt.Errorf("failed to switch mode: %w", err)
		}
	}
	return nil
}

func (r *ElastiServiceReconciler) handleResolverChanges(ctx context.Context, obj interface{}, serviceName, namespace string) error {
	resolverDeployment := &appsv1.Deployment{}
	err := k8sHelper.UnstructuredToResource(obj, resolverDeployment)
	if err != nil {
		return fmt.Errorf("failed to convert unstructured to deployment: %w", err)
	}
	if resolverDeployment.Name == resolverDeploymentName {
		targetNamespacedName := types.NamespacedName{
			Name:      serviceName,
			Namespace: namespace,
		}
		targetSVC := &v1.Service{}
		if err := r.Get(ctx, targetNamespacedName, targetSVC); err != nil {
			return fmt.Errorf("failed to get service to update endpointslice: %w", err)
		}
		if err := r.createOrUpdateEndpointsliceToResolver(ctx, targetSVC); err != nil {
			return fmt.Errorf("failed to create or update endpointslice to resolver: %w", err)
		}
	}
	return nil
}

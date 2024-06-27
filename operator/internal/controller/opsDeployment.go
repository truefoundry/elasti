package controller

import (
	"context"

	"github.com/truefoundry/elasti/pkg/k8sHelper"
	"github.com/truefoundry/elasti/pkg/values"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"truefoundry.io/elasti/api/v1alpha1"
)

func (r *ElastiServiceReconciler) handleTargetDeploymentChanges(ctx context.Context, obj interface{}, es *v1alpha1.ElastiService, req ctrl.Request) {
	targetDeployment := &appsv1.Deployment{}
	err := k8sHelper.UnstructuredToResource(obj, targetDeployment)
	if err != nil {
		r.Logger.Error("Failed to convert unstructured to deployment", zap.Error(err))
		return
	}
	condition := targetDeployment.Status.Conditions
	if targetDeployment.Status.Replicas == 0 {
		r.Logger.Debug("Deployment has 0 replicas", zap.String("deployment_name", es.Spec.DeploymentName), zap.String("es", req.String()))
		_, err := r.switchMode(ctx, req, values.ProxyMode)
		if err != nil {
			r.Logger.Error("Reconciliation failed", zap.String("es", req.String()), zap.Error(err))
			return
		}
	} else if targetDeployment.Status.Replicas > 0 && condition[1].Status == values.DeploymentConditionStatusTrue {
		r.Logger.Debug("Deployment has replicas", zap.String("deployment_name", es.Spec.DeploymentName), zap.String("es", req.String()))
		_, err := r.switchMode(ctx, req, values.ServeMode)
		if err != nil {
			r.Logger.Error("Reconciliation failed", zap.String("es", req.String()), zap.Error(err))
			return
		}
	}
	r.Logger.Info("Deployment changes handled", zap.String("deployment_name", es.Spec.DeploymentName), zap.String("es", req.String()))
}

func (r *ElastiServiceReconciler) handleResolverChanges(ctx context.Context, obj interface{}, serviceName, namespace string) {
	resolverDeployment := &appsv1.Deployment{}
	err := k8sHelper.UnstructuredToResource(obj, resolverDeployment)
	if err != nil {
		r.Logger.Error("Failed to convert unstructured to deployment", zap.Error(err))
		return
	}
	if resolverDeployment.Name == resolverDeploymentName {
		targetNamespacedName := types.NamespacedName{
			Name:      serviceName,
			Namespace: namespace,
		}
		targetSVC := &v1.Service{}
		if err := r.Get(ctx, targetNamespacedName, targetSVC); err != nil {
			r.Logger.Error("Failed to get service to update endpointslice", zap.String("service", targetNamespacedName.String()), zap.Error(err))
			return
		}
		if err := r.createOrUpdateEndpointsliceToResolver(ctx, targetSVC); err != nil {
			r.Logger.Error("Failed to create or update endpointslice to resolver", zap.String("service", targetNamespacedName.String()), zap.Error(err))
			return
		}
	}
	r.Logger.Info("Resolver changes handled", zap.String("deployment_name", resolverDeploymentName))
}

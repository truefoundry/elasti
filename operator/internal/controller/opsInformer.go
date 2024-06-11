package controller

import (
	"context"
	"sync"

	argo "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"

	"truefoundry.io/elasti/api/v1alpha1"
)

const (
	ArgoPhaseHealthy              = "Healthy"
	DeploymentConditionStatusTrue = "True"

	KindDeployments = "Deployments"
	KindRollout     = "Rollout"
)

func (r *ElastiServiceReconciler) getMutexForInformerStart(key string) *sync.Once {
	l, _ := r.WatcherStartLock.LoadOrStore(key, &sync.Once{})
	return l.(*sync.Once)
}

func (r *ElastiServiceReconciler) resetMutexForInformer(key string) {
	r.WatcherStartLock.Delete(key)
}

func (r *ElastiServiceReconciler) getResolverChangeHandler(ctx context.Context, es *v1alpha1.ElastiService, req ctrl.Request) cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			r.handleResolverChanges(ctx, obj, es.Spec.Service, req.Namespace)
		},
		UpdateFunc: func(old, new interface{}) {
			r.handleResolverChanges(ctx, new, es.Spec.Service, req.Namespace)
		},
		DeleteFunc: func(obj interface{}) {
			// TODO: Handle deletion of resolver deployment
			// We can do two things here
			// 1. We can move to the serve mode
			// 2. We can add a finalizer to the deployent to avoid deletion
			//
			//
			// Another situation is, if the resolver has some issues, and is restarting.
			// In that case, we can wait for the resolver to come back up, and in the meanwhile, we can move to the serve mode
			// Or don't do anything
			r.Logger.Debug("Resolver deployment deleted", zap.String("deployment_name", resolverDeploymentName), zap.String("es", req.String()))
		},
	}
}

func (r *ElastiServiceReconciler) getPublicServiceChangeHandler(ctx context.Context, es *v1alpha1.ElastiService, req ctrl.Request) cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			r.handlePublicServiceChanges(ctx, obj, es.Spec.Service, req.Namespace)
		},
		UpdateFunc: func(old, new interface{}) {
			r.handlePublicServiceChanges(ctx, new, es.Spec.Service, req.Namespace)
		},
		DeleteFunc: func(obj interface{}) {
			r.Logger.Debug("public deployment deleted",
				zap.String("es", req.String()),
				zap.String("service", es.Spec.Service))
		},
	}
}

func (r *ElastiServiceReconciler) getScaleTargetRefChangeHandler(ctx context.Context, es *v1alpha1.ElastiService, req ctrl.Request) cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, new interface{}) {
			r.handleScaleTargetRefChanges(ctx, new, es, req)
		},
	}
}

func (r *ElastiServiceReconciler) handleScaleTargetRefChanges(ctx context.Context, obj interface{}, es *v1alpha1.ElastiService, req ctrl.Request) {
	switch es.Spec.ScaleTargetRef.Kind {
	case KindDeployments:
		r.Logger.Info("ScaleTargetRef kind is deployment", zap.String("kind", es.Spec.ScaleTargetRef.Kind))
		r.handleTargetDeploymentChanges(ctx, obj, es, req)
	case KindRollout:
		r.Logger.Info("ScaleTargetRef kind is rollout", zap.String("kind", es.Spec.ScaleTargetRef.Kind))
		r.handleTargetRolloutChanges(ctx, obj, es, req)
	default:
		r.Logger.Error("Unsupported target kind", zap.String("kind", es.Spec.ScaleTargetRef.Kind))
	}
}

func (r *ElastiServiceReconciler) handleTargetRolloutChanges(ctx context.Context, obj interface{}, es *v1alpha1.ElastiService, req ctrl.Request) {
	newRollout := &argo.Rollout{}
	err := r.unstructuredToResource(obj, newRollout)
	if err != nil {
		r.Logger.Error("Failed to convert unstructured to rollout", zap.Error(err))
		return
	}
	replicas := newRollout.Status.ReadyReplicas
	condition := newRollout.Status.Phase
	if replicas == 0 {
		r.Logger.Debug("Rollout has 0 replicas", zap.String("rollout_name", es.Spec.ScaleTargetRef.Name), zap.String("es", req.String()))
		_, err := r.runReconcile(ctx, req, ProxyMode)
		if err != nil {
			r.Logger.Error("Reconciliation failed", zap.String("es", req.String()), zap.Error(err))
			return
		}
	} else if replicas > 0 && condition == ArgoPhaseHealthy {
		r.Logger.Debug("Rollout has replicas", zap.String("rollout_name", es.Spec.ScaleTargetRef.Name), zap.String("es", req.String()))
		_, err := r.runReconcile(ctx, req, ServeMode)
		if err != nil {
			r.Logger.Error("Reconciliation failed", zap.String("es", req.String()), zap.Error(err))
			return
		}
	}
	r.Logger.Info("Rollout changes handled", zap.String("rollout_name", es.Spec.ScaleTargetRef.Name), zap.String("es", req.String()))
}

func (r *ElastiServiceReconciler) handleTargetDeploymentChanges(ctx context.Context, obj interface{}, es *v1alpha1.ElastiService, req ctrl.Request) {
	newDeployment := &appsv1.Deployment{}
	err := r.unstructuredToResource(obj, newDeployment)
	if err != nil {
		r.Logger.Error("Failed to convert unstructured to deployment", zap.Error(err))
		return
	}
	condition := newDeployment.Status.Conditions
	if newDeployment.Status.Replicas == 0 {
		r.Logger.Debug("Deployment has 0 replicas", zap.String("deployment_name", es.Spec.DeploymentName), zap.String("es", req.String()))
		_, err := r.runReconcile(ctx, req, ProxyMode)
		if err != nil {
			r.Logger.Error("Reconciliation failed", zap.String("es", req.String()), zap.Error(err))
			return
		}
	} else if newDeployment.Status.Replicas > 0 && condition[1].Status == DeploymentConditionStatusTrue {
		r.Logger.Debug("Deployment has replicas", zap.String("deployment_name", es.Spec.DeploymentName), zap.String("es", req.String()))
		_, err := r.runReconcile(ctx, req, ServeMode)
		if err != nil {
			r.Logger.Error("Reconciliation failed", zap.String("es", req.String()), zap.Error(err))
			return
		}
	}
	r.Logger.Info("Deployment changes handled", zap.String("deployment_name", es.Spec.DeploymentName), zap.String("es", req.String()))
}

func (r *ElastiServiceReconciler) handlePublicServiceChanges(_ context.Context, obj interface{}, _, _ string) {
	publicService := &v1.Service{}
	err := r.unstructuredToResource(obj, publicService)
	if err != nil {
		r.Logger.Error("Failed to convert unstructured to service", zap.Error(err))
		return
	}

	// if publicService.Name == serviceName {
	// 	targetNamespacedName := types.NamespacedName{
	// 		Name:      serviceName,
	// 		Namespace: namespace,
	// 	}
	// 	targetSVC := &v1.Service{}
	// 	if err := r.Get(ctx, targetNamespacedName, targetSVC); err != nil {
	// 		r.Logger.Error("Failed to get service to update endpointslice", zap.String("service", targetNamespacedName.String()), zap.Error(err))
	// 		return
	// 	}
	// 	if err := r.createOrUpdateEndpointsliceToResolver(ctx, targetSVC); err != nil {
	// 		r.Logger.Error("Failed to create or update endpointslice to resolver", zap.String("service", targetNamespacedName.String()), zap.Error(err))
	// 		return
	// 	}
	// }
	r.Logger.Info("Public service changed", zap.String("service", publicService.Name))
}

func (r *ElastiServiceReconciler) handleResolverChanges(ctx context.Context, obj interface{}, serviceName, namespace string) {
	resolverDeployment := &appsv1.Deployment{}
	err := r.unstructuredToResource(obj, resolverDeployment)
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

func (r *ElastiServiceReconciler) unstructuredToResource(obj interface{}, resource interface{}) error {
	unstructuredObj := obj.(*unstructured.Unstructured)
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.UnstructuredContent(), resource)
	if err != nil {
		r.Logger.Error("Failed to convert unstructured to interface", zap.Error(err))
		return err
	}
	return nil
}

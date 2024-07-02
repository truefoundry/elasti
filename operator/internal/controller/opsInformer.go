package controller

import (
	"context"
	"strings"
	"sync"

	"github.com/truefoundry/elasti/pkg/values"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"

	"truefoundry.io/elasti/api/v1alpha1"
)

const (
	// Prefix is the name of the NamespacedName string for CRD
	lockKeyPostfixForPublicSVC = "public-service"
	lockKeyPostfixForTargetRef = "scale-target-ref"
)

func (r *ElastiServiceReconciler) getMutexForInformerStart(key string) *sync.Once {
	l, _ := r.InformerStartLocks.LoadOrStore(key, &sync.Once{})
	return l.(*sync.Once)
}

func (r *ElastiServiceReconciler) resetMutexForInformer(key string) {
	r.InformerStartLocks.Delete(key)
}

func (r *ElastiServiceReconciler) getMutexKeyForPublicSVC(req ctrl.Request) string {
	return req.String() + lockKeyPostfixForPublicSVC
}

func (r *ElastiServiceReconciler) getMutexKeyForTargetRef(req ctrl.Request) string {
	return req.String() + lockKeyPostfixForTargetRef
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
	switch strings.ToLower(es.Spec.ScaleTargetRef.Kind) {
	case values.KindDeployments:
		r.Logger.Info("ScaleTargetRef kind is deployment", zap.String("kind", es.Spec.ScaleTargetRef.Kind))
		r.handleTargetDeploymentChanges(ctx, obj, es, req)
	case values.KindRollout:
		r.Logger.Info("ScaleTargetRef kind is rollout", zap.String("kind", es.Spec.ScaleTargetRef.Kind))
		r.handleTargetRolloutChanges(ctx, obj, es, req)
	default:
		r.Logger.Error("Unsupported target kind", zap.String("kind", es.Spec.ScaleTargetRef.Kind))
	}
}

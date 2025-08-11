package controller

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/truefoundry/elasti/pkg/values"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/truefoundry/elasti/operator/api/v1alpha1"
	"github.com/truefoundry/elasti/operator/internal/informer"
	"github.com/truefoundry/elasti/operator/internal/prom"
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
func (r *ElastiServiceReconciler) getResolverChangeHandler(ctx context.Context) cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			err := r.handleResolverChanges(ctx, obj)
			if err != nil {
				r.Logger.Error("Failed to handle resolver changes", zap.Error(err))
			}
		},
		UpdateFunc: func(_, newObj interface{}) {
			err := r.handleResolverChanges(ctx, newObj)
			if err != nil {
				r.Logger.Error("Failed to handle resolver changes", zap.Error(err))
			}
		},
		DeleteFunc: func(_ interface{}) {
			// TODO: Handle deletion of resolver deployment
			// We can do two things here
			// 1. We can move to the serve mode
			// 2. We can add a finalizer to the deployent to avoid deletion
			//
			//
			// Another situation is, if the resolver has some issues, and is restarting.
			// In that case, we can wait for the resolver to come back up, and in the meanwhile, we can move to the serve mode
			r.Logger.Warn("Resolver deployment deleted", zap.String("deployment_name", resolverDeploymentName))
		},
	}
}

func (r *ElastiServiceReconciler) getPublicServiceChangeHandler(ctx context.Context, es *v1alpha1.ElastiService, req ctrl.Request) cache.ResourceEventHandlerFuncs {
	key := r.InformerManager.GetKey(informer.KeyParams{
		Namespace:    resolverNamespace,
		CRDName:      req.Name,
		ResourceName: es.Spec.Service,
		Resource:     values.KindService,
	})

	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			errStr := values.Success
			err := r.handlePublicServiceChanges(ctx, obj, es.Spec.Service, req.Namespace)
			if err != nil {
				errStr = err.Error()
				r.Logger.Error("Failed to handle public service changes", zap.Error(err))
			} else {
				r.Logger.Info("Public service added", zap.String("service", es.Spec.Service), zap.String("es", req.String()))
			}
			prom.InformerHandlerCounter.WithLabelValues(req.String(), key, errStr).Inc()
		},
		UpdateFunc: func(_, newObj interface{}) {
			errStr := values.Success
			err := r.handlePublicServiceChanges(ctx, newObj, es.Spec.Service, req.Namespace)
			if err != nil {
				errStr = err.Error()
				r.Logger.Error("Failed to handle public service changes", zap.Error(err))
			} else {
				r.Logger.Info("Public service updated", zap.String("service", es.Spec.Service), zap.String("es", req.String()))
			}
			prom.InformerHandlerCounter.WithLabelValues(req.String(), key, errStr).Inc()
		},
		DeleteFunc: func(_ interface{}) {
			r.Logger.Debug("public deployment deleted",
				zap.String("es", req.String()),
				zap.String("service", es.Spec.Service))
		},
	}
}

func (r *ElastiServiceReconciler) getScaleTargetRefChangeHandler(ctx context.Context, es *v1alpha1.ElastiService, req ctrl.Request) cache.ResourceEventHandlerFuncs {
	key := r.InformerManager.GetKey(informer.KeyParams{
		Namespace:    req.Namespace,
		CRDName:      req.Name,
		ResourceName: es.Spec.ScaleTargetRef.Kind,
		Resource:     es.Spec.ScaleTargetRef.Name,
	})
	return cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(_, newObj interface{}) {
			errStr := values.Success
			err := r.handleScaleTargetRefChanges(ctx, newObj, es, req)
			if err != nil {
				errStr = err.Error()
				r.Logger.Error("Failed to handle ScaleTargetRef changes", zap.Error(err))
			} else {
				r.Logger.Info("ScaleTargetRef updated", zap.String("es", req.String()), zap.String("scaleTargetRef", es.Spec.ScaleTargetRef.Name))
			}

			prom.InformerHandlerCounter.WithLabelValues(req.String(), key, errStr).Inc()
		},
	}
}

func (r *ElastiServiceReconciler) handleScaleTargetRefChanges(ctx context.Context, obj interface{}, es *v1alpha1.ElastiService, req ctrl.Request) error {
	r.Logger.Info("ScaleTargetRef changes detected", zap.String("es", req.String()), zap.Any("scaleTargetRef", es.Spec.ScaleTargetRef))
	switch strings.ToLower(es.Spec.ScaleTargetRef.Kind) {
	case values.KindDeployments:
		return r.handleTargetDeploymentChanges(ctx, obj, es, req)
	case values.KindRollout:
		return r.handleTargetRolloutChanges(ctx, obj, es, req)
	default:
		return fmt.Errorf("unsupported target kind: %s", es.Spec.ScaleTargetRef.Kind)
	}
}

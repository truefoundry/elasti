package controller

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/truefoundry/elasti/pkg/values"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	"truefoundry/elasti/operator/api/v1alpha1"
	"truefoundry/elasti/operator/internal/informer"
	"truefoundry/elasti/operator/internal/prom"
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

func (r *ElastiServiceReconciler) getMutexKeyForPublicSVC(namespacedName types.NamespacedName) string {
	return namespacedName.String() + lockKeyPostfixForPublicSVC
}

func (r *ElastiServiceReconciler) getMutexKeyForTargetRef(namespacedName types.NamespacedName) string {
	return namespacedName.String() + lockKeyPostfixForTargetRef
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

func (r *ElastiServiceReconciler) getPublicServiceChangeHandler(ctx context.Context, es *v1alpha1.ElastiService, namespacedName types.NamespacedName) cache.ResourceEventHandlerFuncs {
	key := r.InformerManager.GetKey(informer.KeyParams{
		Namespace:    resolverNamespace,
		CRDName:      es.GetObjectMeta().GetName(),
		ResourceName: es.Spec.Service,
		Resource:     values.KindService,
	})

	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			errStr := values.Success
			err := r.handlePublicServiceChanges(ctx, obj, es.Spec.Service, es.GetObjectMeta().GetNamespace())
			if err != nil {
				errStr = err.Error()
				r.Logger.Error("Failed to handle public service changes", zap.Error(err))
			} else {
				r.Logger.Info("Public service added", zap.String("service", es.Spec.Service), zap.String("es", namespacedName.String()))
			}
			prom.InformerHandlerCounter.WithLabelValues(namespacedName.String(), key, errStr).Inc()
		},
		UpdateFunc: func(_, newObj interface{}) {
			errStr := values.Success
			err := r.handlePublicServiceChanges(ctx, newObj, es.Spec.Service, es.GetObjectMeta().GetNamespace())
			if err != nil {
				errStr = err.Error()
				r.Logger.Error("Failed to handle public service changes", zap.Error(err))
			} else {
				r.Logger.Info("Public service updated", zap.String("service", es.Spec.Service), zap.String("es", namespacedName.String()))
			}
			prom.InformerHandlerCounter.WithLabelValues(namespacedName.String(), key, errStr).Inc()
		},
		DeleteFunc: func(_ interface{}) {
			r.Logger.Debug("public service deleted",
				zap.String("es", namespacedName.String()),
				zap.String("service", es.Spec.Service))
		},
	}
}

func (r *ElastiServiceReconciler) getScaleTargetRefChangeHandler(ctx context.Context, es *v1alpha1.ElastiService, elastiServiceNamespacedName types.NamespacedName) cache.ResourceEventHandlerFuncs {
	key := r.InformerManager.GetKey(informer.KeyParams{
		Namespace:    elastiServiceNamespacedName.Namespace,
		CRDName:      elastiServiceNamespacedName.Name,
		ResourceName: es.Spec.ScaleTargetRef.Kind,
		Resource:     es.Spec.ScaleTargetRef.Name,
	})
	return cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(_, newObj interface{}) {
			errStr := values.Success
			err := r.handleScaleTargetRefChanges(ctx, newObj, elastiServiceNamespacedName, es)
			if err != nil {
				errStr = err.Error()
				r.Logger.Error("Failed to handle ScaleTargetRef changes", zap.Error(err))
			} else {
				r.Logger.Info("ScaleTargetRef updated", zap.String("es", elastiServiceNamespacedName.String()), zap.String("scaleTargetRef", es.Spec.ScaleTargetRef.Name))
			}

			prom.InformerHandlerCounter.WithLabelValues(elastiServiceNamespacedName.String(), key, errStr).Inc()
		},
	}
}

func (r *ElastiServiceReconciler) handleScaleTargetRefChanges(ctx context.Context, obj interface{}, elastiServiceNamespacedName types.NamespacedName, es *v1alpha1.ElastiService) error {
	r.Logger.Info("ScaleTargetRef changes detected", zap.String("es", elastiServiceNamespacedName.String()), zap.Any("scaleTargetRef", es.Spec.ScaleTargetRef))
	switch strings.ToLower(es.Spec.ScaleTargetRef.Kind) {
	case values.KindDeployments:
		return r.handleTargetDeploymentChanges(ctx, obj, elastiServiceNamespacedName, es)
	case values.KindRollout:
		return r.handleTargetRolloutChanges(ctx, obj, elastiServiceNamespacedName, es)
	default:
		return fmt.Errorf("unsupported target kind: %s", es.Spec.ScaleTargetRef.Kind)
	}
}

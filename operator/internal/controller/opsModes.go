package controller

import (
	"context"
	"sync"

	"github.com/truefoundry/elasti/pkg/values"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"truefoundry.io/elasti/api/v1alpha1"
)

func (r *ElastiServiceReconciler) getMutexForSwitchMode(key string) *sync.Mutex {
	l, _ := r.SwitchModeLocks.LoadOrStore(key, &sync.Mutex{})
	return l.(*sync.Mutex)
}

func (r *ElastiServiceReconciler) switchMode(ctx context.Context, req ctrl.Request, mode string) (res ctrl.Result, err error) {
	r.Logger.Debug("- In SwitchMode", zap.String("es", req.NamespacedName.String()))
	// Only 1 switchMode should run at a time for a given ElastiService. This prevents conflicts when updating different objects.
	mutex := r.getMutexForSwitchMode(req.NamespacedName.String())
	mutex.Lock()
	defer r.Logger.Debug("- Out of SwitchMode", zap.String("es", req.NamespacedName.String()))
	defer mutex.Unlock()
	es, err := r.getCRD(ctx, req.NamespacedName)
	defer r.updateCRDStatus(ctx, req.NamespacedName, mode)
	switch mode {
	case values.ServeMode:
		if err = r.enableServeMode(ctx, req, es); err != nil {
			r.Logger.Error("Failed to enable serve mode", zap.String("es", req.NamespacedName.String()), zap.Error(err))
			return res, err
		}
		r.Logger.Info("Serve mode enabled", zap.String("es", req.NamespacedName.String()))
	case values.ProxyMode:
		if err = r.enableProxyMode(ctx, req, es); err != nil {
			r.Logger.Error("Failed to enable proxy mode", zap.String("es", req.NamespacedName.String()), zap.Error(err))
			return res, err
		}
		r.Logger.Debug("Proxy mode enabled", zap.String("es", req.NamespacedName.String()))
	default:
		r.Logger.Error("Invalid mode", zap.String("mode", mode), zap.String("es", req.NamespacedName.String()))
	}
	return res, nil
}

func (r *ElastiServiceReconciler) enableProxyMode(ctx context.Context, req ctrl.Request, es *v1alpha1.ElastiService) error {
	targetNamespacedName := types.NamespacedName{
		Name:      es.Spec.Service,
		Namespace: es.Namespace,
	}
	targetSVC := &v1.Service{}
	if err := r.Get(ctx, targetNamespacedName, targetSVC); err != nil {
		r.Logger.Error("Failed to get target service", zap.String("service", targetNamespacedName.String()), zap.Error(err))
		return err
	}
	_, err := r.checkAndCreatePrivateService(ctx, targetSVC, es)
	if err != nil {
		return err
	}

	// Check if Public Service is present, and has not changed from the values in CRDDirectory
	if err := r.checkChangesInPublicService(ctx, es, req); err != nil {
		r.Logger.Error("Failed to check changes in public service", zap.String("es", req.String()), zap.Error(err))
		return err
	}

	if err = r.createOrUpdateEndpointsliceToResolver(ctx, targetSVC); err != nil {
		return err
	}

	// Watch for changes in resolver deployment, and update the endpointslice since we are in proxy mode
	r.Informer.AddDeploymentWatch(req, resolverDeploymentName, resolverNamespace, r.getResolverChangeHandler(ctx, es, req))

	return nil
}

func (r *ElastiServiceReconciler) enableServeMode(ctx context.Context, req ctrl.Request, es *v1alpha1.ElastiService) error {
	// Stop the watch on resolver deployment, since we are in serve mode
	key := r.Informer.GetKey(resolverNamespace, req.Name, resolverDeploymentName, values.KindDeployments)
	r.Informer.StopInformer(key)

	targetNamespacedName := types.NamespacedName{
		Name:      es.Spec.Service,
		Namespace: es.Namespace,
	}
	if err := r.deleteEndpointsliceToResolver(ctx, targetNamespacedName); err != nil {
		return err
	}
	return nil
}

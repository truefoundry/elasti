package controller

import (
	"context"
	"sync"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"truefoundry.io/elasti/api/v1alpha1"
)

func (r *ElastiServiceReconciler) getMutexForRunReconcile(key string) *sync.Mutex {
	l, _ := r.RunReconcileLocks.LoadOrStore(key, &sync.Mutex{})
	return l.(*sync.Mutex)
}

func (r *ElastiServiceReconciler) runReconcile(ctx context.Context, req ctrl.Request, mode string) (res ctrl.Result, err error) {
	r.Logger.Debug("- In RunReconcile", zap.String("es", req.NamespacedName.String()))
	// Only 1 reconcile should run at a time for a given ElastiService. This prevents conflicts when updating different objects.
	mutex := r.getMutexForRunReconcile(req.NamespacedName.String())
	mutex.Lock()
	defer r.Logger.Debug("- Out of RunReconcile", zap.String("es", req.NamespacedName.String()))
	defer mutex.Unlock()

	es, err := r.getCRD(ctx, req.NamespacedName)

	if mode != ProxyMode && mode != ServeMode {
		nam := types.NamespacedName{
			Name:      es.Spec.DeploymentName,
			Namespace: req.Namespace,
		}
		mode, err = r.getModeFromDeployment(ctx, nam)
		if err != nil {
			r.Logger.Error("Failed to get mode from deployment", zap.String("es", req.NamespacedName.String()), zap.Error(err))
			return res, err
		}
	}
	defer r.updateCRDStatus(ctx, req.NamespacedName, mode)

	switch mode {
	case ServeMode:
		if err = r.enableServeMode(ctx, es); err != nil {
			r.Logger.Error("Failed to enable serve mode", zap.String("es", req.NamespacedName.String()), zap.Error(err))
			return res, err
		}
		r.Logger.Info("Serve mode enabled", zap.String("es", req.NamespacedName.String()))
	case ProxyMode:
		if err = r.enableProxyMode(ctx, es); err != nil {
			r.Logger.Error("Failed to enable proxy mode", zap.String("es", req.NamespacedName.String()), zap.Error(err))
			return res, err
		}
		r.Logger.Debug("Proxy mode enabled", zap.String("es", req.NamespacedName.String()))
	}

	return res, nil
}

func (r *ElastiServiceReconciler) enableProxyMode(ctx context.Context, es *v1alpha1.ElastiService) error {
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
	if err = r.createOrUpdateEndpointsliceToResolver(ctx, targetSVC); err != nil {
		return err
	}
	return nil
}

func (r *ElastiServiceReconciler) enableServeMode(ctx context.Context, es *v1alpha1.ElastiService) error {
	targetNamespacedName := types.NamespacedName{
		Name:      es.Spec.Service,
		Namespace: es.Namespace,
	}
	if err := r.deleteEndpointsliceToResolver(ctx, targetNamespacedName); err != nil {
		return err
	}
	return nil
}

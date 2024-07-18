package controller

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"truefoundry/elasti/operator/api/v1alpha1"
	"truefoundry/elasti/operator/internal/informer"

	"github.com/truefoundry/elasti/pkg/values"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *ElastiServiceReconciler) getMutexForSwitchMode(key string) *sync.Mutex {
	l, _ := r.SwitchModeLocks.LoadOrStore(key, &sync.Mutex{})
	return l.(*sync.Mutex)
}

func (r *ElastiServiceReconciler) switchMode(ctx context.Context, req ctrl.Request, mode string) (res ctrl.Result, err error) {
	{
		r.Logger.Debug(fmt.Sprintf("[Switching to %s Mode]", strings.ToUpper(mode)), zap.String("es", req.NamespacedName.String()))
		mutex := r.getMutexForSwitchMode(req.NamespacedName.String())
		mutex.Lock()
		defer mutex.Unlock()
	}

	es, err := r.getCRD(ctx, req.NamespacedName)
	if err != nil {
		r.Logger.Error("Failed to get CRD", zap.String("es", req.NamespacedName.String()), zap.Error(err))
		return res, fmt.Errorf("failed to get CRD: %w", err)
	}
	defer r.updateCRDStatus(ctx, req.NamespacedName, mode)
	switch mode {
	case values.ServeMode:
		if es.Status.Mode != values.ServeMode {
			if err = r.enableServeMode(ctx, req, es); err != nil {
				r.Logger.Error("Failed to enable SERVE mode", zap.String("es", req.NamespacedName.String()), zap.Error(err))
				return res, err
			}
			r.Logger.Info("[SERVE mode enabled]", zap.String("es", req.NamespacedName.String()))
		}
	case values.ProxyMode:
		if es.Status.Mode != values.ProxyMode {
			if err = r.enableProxyMode(ctx, req, es); err != nil {
				r.Logger.Error("Failed to enable PROXY mode", zap.String("es", req.NamespacedName.String()), zap.Error(err))
				return res, err
			}
			r.Logger.Info("[PROXY mode enabled]", zap.String("es", req.NamespacedName.String()))
		}
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
		return fmt.Errorf("failed to get target service: %w", err)
	}
	PVTName, err := r.checkAndCreatePrivateService(ctx, targetSVC, es)
	if err != nil {
		return fmt.Errorf("failed to check and create private service: %w", err)
	}
	r.Logger.Info("1. Checked and created private service", zap.String("public service", targetSVC.Name), zap.String("private service", PVTName))

	// Check if Public Service is present, and has not changed from the values in CRDDirectory
	if err := r.watchPublicService(ctx, es, req); err != nil {
		return fmt.Errorf("failed to add watch on public service: %w", err)
	}
	r.Logger.Info("2. Added watch on public service", zap.String("service", targetSVC.Name))

	if err = r.createOrUpdateEndpointsliceToResolver(ctx, targetSVC); err != nil {
		return fmt.Errorf("failed to create or update endpointslice to resolver: %w ", err)
	}
	r.Logger.Info("3. Created or updated endpointslice to resolver", zap.String("service", targetSVC.Name))

	// Watch for changes in resolver deployment, and update the endpointslice since we are in proxy mode
	if err := r.Informer.WatchDeployment(req, resolverDeploymentName, resolverNamespace, r.getResolverChangeHandler(ctx, es, req)); err != nil {
		return fmt.Errorf("failed to add watch on resolver deployment: %w", err)
	}
	r.Logger.Info("4. Added watch on resolver deployment", zap.String("deployment", resolverDeploymentName))

	return nil
}

func (r *ElastiServiceReconciler) enableServeMode(ctx context.Context, req ctrl.Request, es *v1alpha1.ElastiService) error {
	// Stop the watch on resolver deployment, since we are in serve mode
	key := r.Informer.GetKey(informer.KeyParams{
		Namespace:    resolverNamespace,
		CRDName:      req.Name,
		ResourceName: resolverDeploymentName,
		Resource:     values.KindDeployments,
	})
	err := r.Informer.StopInformer(key)
	if err != nil {
		r.Logger.Error("Failed to stop watch on resolver deployment", zap.String("deployment", resolverDeploymentName), zap.Error(err))
	}
	r.Logger.Info("1. Stopped watch on resolver deployment", zap.String("deployment", resolverDeploymentName))

	targetNamespacedName := types.NamespacedName{
		Name:      es.Spec.Service,
		Namespace: es.Namespace,
	}
	if err := r.deleteEndpointsliceToResolver(ctx, targetNamespacedName); err != nil {
		return fmt.Errorf("failed to delete endpointslice to resolver: %w", err)
	}
	r.Logger.Info("2. Deleted endpointslice to resolver", zap.String("service", targetNamespacedName.String()))
	return nil
}

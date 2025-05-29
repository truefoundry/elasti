package controller

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"truefoundry/elasti/operator/api/v1alpha1"

	"github.com/truefoundry/elasti/pkg/values"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *ElastiServiceReconciler) getMutexForSwitchMode(key string) *sync.Mutex {
	l, _ := r.SwitchModeLocks.LoadOrStore(key, &sync.Mutex{})
	return l.(*sync.Mutex)
}

func (r *ElastiServiceReconciler) switchMode(ctx context.Context, elastiServiceNamespacedName types.NamespacedName, mode string) error {
	{
		r.Logger.Debug(fmt.Sprintf("[Switching to %s Mode]", strings.ToUpper(mode)), zap.String("es", elastiServiceNamespacedName.String()))
		mutex := r.getMutexForSwitchMode(elastiServiceNamespacedName.String())
		mutex.Lock()
		defer mutex.Unlock()
	}

	es, err := r.getCRD(ctx, elastiServiceNamespacedName)
	if err != nil {
		r.Logger.Error("Failed to get CRD", zap.String("es", elastiServiceNamespacedName.String()), zap.Error(err))
		return fmt.Errorf("failed to get CRD: %w", err)
	}

	//nolint: errcheck
	defer r.updateCRDStatus(ctx, elastiServiceNamespacedName, mode)
	switch mode {
	case values.ServeMode:
		if err = r.enableServeMode(ctx, elastiServiceNamespacedName, es); err != nil {
			r.Logger.Error("Failed to enable SERVE mode", zap.String("es", elastiServiceNamespacedName.String()), zap.Error(err))
			return err
		}
		r.Logger.Info("[SERVE mode enabled]", zap.String("es", elastiServiceNamespacedName.String()))
	case values.ProxyMode:
		if err = r.enableProxyMode(ctx, elastiServiceNamespacedName, es); err != nil {
			r.Logger.Error("Failed to enable PROXY mode", zap.String("es", elastiServiceNamespacedName.String()), zap.Error(err))
			return err
		}
		r.Logger.Info("[PROXY mode enabled]", zap.String("es", elastiServiceNamespacedName.String()))
	default:
		r.Logger.Error("Invalid mode", zap.String("mode", mode), zap.String("es", elastiServiceNamespacedName.String()))
	}
	return nil
}

func (r *ElastiServiceReconciler) enableProxyMode(ctx context.Context, elastiServiceNamespacedName types.NamespacedName, es *v1alpha1.ElastiService) error {
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
	if err := r.watchPublicService(ctx, es, elastiServiceNamespacedName); err != nil {
		return fmt.Errorf("failed to add watch on public service: %w", err)
	}
	r.Logger.Info("2. Added watch on public service", zap.String("service", targetSVC.Name))

	if err = r.createOrUpdateEndpointsliceToResolver(ctx, targetSVC); err != nil {
		return fmt.Errorf("failed to create or update endpointslice to resolver: %w ", err)
	}
	r.Logger.Info("3. Created or updated endpointslice to resolver", zap.String("service", targetSVC.Name))

	return nil
}

func (r *ElastiServiceReconciler) enableServeMode(ctx context.Context, elastiServiceNamespacedName types.NamespacedName, es *v1alpha1.ElastiService) error {
	if err := r.deleteEndpointsliceToResolver(ctx, elastiServiceNamespacedName); err != nil {
		return fmt.Errorf("failed to delete endpointslice to resolver: %w", err)
	}
	r.Logger.Info("1. Deleted endpointslice to resolver", zap.String("service", es.Spec.Service))
	return nil
}

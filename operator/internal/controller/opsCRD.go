package controller

import (
	"context"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"truefoundry.io/elasti/api/v1alpha1"
	"truefoundry.io/elasti/internal/crdDirectory"
)

func (r *ElastiServiceReconciler) getCRD(ctx context.Context, crdNamespacedName types.NamespacedName) (*v1alpha1.ElastiService, error) {
	es := &v1alpha1.ElastiService{}
	if err := r.Get(ctx, crdNamespacedName, es); err != nil {
		r.Logger.Error("Failed to get ElastiService", zap.String("es", crdNamespacedName.String()), zap.Error(err))
		return nil, err
	}
	return es, nil
}

func (r *ElastiServiceReconciler) updateCRDStatus(ctx context.Context, crdNamespacedName types.NamespacedName, mode string) {
	es := &v1alpha1.ElastiService{}
	if err := r.Client.Get(ctx, crdNamespacedName, es); err != nil {
		r.Logger.Error("Failed to get ElastiService for status update", zap.String("es", crdNamespacedName.String()), zap.Error(err))
		return
	}
	es.Status.LastReconciledTime = metav1.Now()
	es.Status.Mode = mode
	if err := r.Status().Update(ctx, es); err != nil {
		r.Logger.Error("Failed to update status", zap.String("es", crdNamespacedName.String()), zap.Error(err))
		return
	}
	r.Logger.Info("CRD Status updated successfully")
}

func (r *ElastiServiceReconciler) checkFinalizerCRD(ctx context.Context, es *v1alpha1.ElastiService, req ctrl.Request) error {
	// If the ElastiService is being deleted, we need to clean up the resources
	if !es.ObjectMeta.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(es, v1alpha1.ElastiServiceFinalizer) {
			// If CRD contains finalizer, we call the finaizer function and remove the finalizer post that
			if err := r.finalizeCRD(ctx, es, req); err != nil {
				r.Logger.Error("Failed to enable serve mode", zap.String("es", req.String()), zap.Error(err))
				return err
			}
			controllerutil.RemoveFinalizer(es, v1alpha1.ElastiServiceFinalizer)
			if err := r.Update(ctx, es); err != nil {
				return err
			}
		}
		return nil
	}
	// If the CRD does not contain the finalizer, we add the finalizer
	if !controllerutil.ContainsFinalizer(es, v1alpha1.ElastiServiceFinalizer) {
		controllerutil.AddFinalizer(es, v1alpha1.ElastiServiceFinalizer)
		if err := r.Update(ctx, es); err != nil {
			r.Logger.Error("Failed to add finalizer", zap.String("es", req.String()), zap.Error(err))
			return err
		} else {
			r.Logger.Info("Finalizer added", zap.String("es", req.String()))
		}
	}
	return nil
}

func (r *ElastiServiceReconciler) finalizeCRD(ctx context.Context, es *v1alpha1.ElastiService, req ctrl.Request) error {
	r.Logger.Info("ElastiService is being deleted", zap.String("name", es.Name), zap.Any("deletionTimestamp", es.ObjectMeta.DeletionTimestamp))
	// Reset the informer start mutex, so if the ElastiService is recreated, we will need to reset the informer
	r.resetMutexForInformer(req.NamespacedName.String())
	// Stop target service informer
	go r.Informer.StopInformer(es.Name, es.Spec.DeploymentName, es.Namespace)
	// Stop resolver informer
	go r.Informer.StopInformer(es.Name, resolverDeploymentName, resolverNamespace)
	// Remove CRD details from service directory
	crdDirectory.CRDDirectory.RemoveCRD(es.Spec.Service)

	targetNamespacedName := types.NamespacedName{
		Name:      es.Spec.Service,
		Namespace: es.Namespace,
	}
	// Delete EndpointSlice to resolver
	if err := r.deleteEndpointsliceToResolver(ctx, targetNamespacedName); err != nil {
		return err
	}
	// Delete private service
	if err := r.deletePrivateService(ctx, targetNamespacedName); err != nil {
		return err
	}
	r.Logger.Info("Serve mode enabled")
	return nil
}

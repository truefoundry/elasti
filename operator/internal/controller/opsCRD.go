package controller

import (
	"context"
	"strings"

	"github.com/truefoundry/elasti/pkg/k8sHelper"
	"github.com/truefoundry/elasti/pkg/utils"
	"github.com/truefoundry/elasti/pkg/values"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"truefoundry.io/elasti/api/v1alpha1"
	"truefoundry.io/elasti/internal/crdDirectory"
	"truefoundry.io/elasti/internal/informer"
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

func (r *ElastiServiceReconciler) checkAndAddCRDFinalizer(ctx context.Context, es *v1alpha1.ElastiService, req ctrl.Request) error {
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
	r.resetMutexForInformer(r.getMutexKeyForTargetRef(req))
	r.resetMutexForInformer(r.getMutexKeyForPublicSVC(req))
	// Stop all active informers
	// NOTE: If the informerManager is shared across multiple controllers, this will stop all informers
	// In that case, we must call the
	go r.Informer.StopForCRD(req.Name)
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

// checkChangesInScaleTargetRef checks if the ScaleTargetRef has changed, and if it has, stops the informer for the old ScaleTargetRef
// Start the new informer for the new ScaleTargetRef
func (r *ElastiServiceReconciler) checkChangesInScaleTargetRef(ctx context.Context, es *v1alpha1.ElastiService, req ctrl.Request) error {
	if es.Spec.ScaleTargetRef.Name == "" ||
		es.Spec.ScaleTargetRef.Kind == "" ||
		es.Spec.ScaleTargetRef.APIVersion == "" {
		r.Logger.Error("ScaleTargetRef is not present", zap.String("es", req.String()), zap.Any("scaleTargetRef", es.Spec.ScaleTargetRef))
		return k8sHelper.ErrNoScaleTargetFound
	}

	crd, found := crdDirectory.CRDDirectory.GetCRD(es.Spec.Service)
	if found {
		if es.Spec.ScaleTargetRef.Name != crd.Spec.ScaleTargetRef.Name ||
			es.Spec.ScaleTargetRef.Kind != crd.Spec.ScaleTargetRef.Kind ||
			es.Spec.ScaleTargetRef.APIVersion != crd.Spec.ScaleTargetRef.APIVersion {
			r.Logger.Info("ScaleTargetRef has changed", zap.String("es", req.String()))
			r.Logger.Debug("Stopping informer for scaleTargetRef", zap.Any("scaleTargetRef", es.Spec.ScaleTargetRef))
			key := r.Informer.GetKey(req.Namespace, req.Name, crd.Spec.ScaleTargetRef.Name, strings.ToLower(crd.Spec.ScaleTargetRef.Kind))
			r.Informer.StopInformer(key)
			r.Logger.Debug("Resetting mutex for scaleTargetRef", zap.Any("scaleTargetRef", es.Spec.ScaleTargetRef))
			r.resetMutexForInformer(r.getMutexKeyForTargetRef(req))
		}
	}

	r.getMutexForInformerStart(r.getMutexKeyForTargetRef(req)).Do(func() {
		targetGroup, targetVersion, err := utils.ParseAPIVersion(es.Spec.ScaleTargetRef.APIVersion)
		if err != nil {
			r.Logger.Error("Failed to parse API version", zap.String("APIVersion", es.Spec.ScaleTargetRef.APIVersion), zap.Error(err))
			return
		}
		r.Informer.Add(&informer.RequestWatch{
			Req:               req,
			ResourceName:      es.Spec.ScaleTargetRef.Name,
			ResourceNamespace: req.Namespace,
			GroupVersionResource: &schema.GroupVersionResource{
				Group:    targetGroup,
				Version:  targetVersion,
				Resource: strings.ToLower(es.Spec.ScaleTargetRef.Kind),
			},
			Handlers: r.getScaleTargetRefChangeHandler(ctx, es, req),
		})

		r.Logger.Info("ScaleTargetRef added to informer", zap.String("es", req.String()),
			zap.String("scaleTargetRef", es.Spec.ScaleTargetRef.Name),
		)
	})
	return nil
}

// checkChangesInPublicService checks if the Public Service has changed, and makes sure it's not null

func (r *ElastiServiceReconciler) checkChangesInPublicService(ctx context.Context, es *v1alpha1.ElastiService, req ctrl.Request) error {
	if es.Spec.Service == "" {
		r.Logger.Error("Public Service is not present", zap.String("es", req.String()))
		return k8sHelper.ErrNoPublicServiceFound
	}

	// crd, found := crdDirectory.CRDDirectory.GetCRD(es.Spec.Service)
	// if found {
	// 	if crd.Spec.Service != es.Spec.Service {
	// 		r.Logger.Info("Public Service has changed", zap.String("es", req.String()))
	// 		r.Logger.Debug("Stopping informer for public service", zap.String("public service", crd.Spec.Service))
	// 		key := r.Informer.GetKey(req.Namespace, req.Name, crd.Spec.Service, values.KindService)
	// 		r.Informer.StopInformer(key)
	// 		r.Logger.Debug("Resetting mutex for public service", zap.String("public service", crd.Spec.Service))
	// 		r.resetMutexForInformer(r.getMutexKeyForPublicSVC(req))
	// 	}
	// }

	r.getMutexForInformerStart(r.getMutexKeyForPublicSVC(req)).Do(func() {
		r.Informer.Add(&informer.RequestWatch{
			Req:                  req,
			ResourceName:         es.Spec.Service,
			ResourceNamespace:    es.Namespace,
			GroupVersionResource: &values.ServiceGVR,
			Handlers:             r.getPublicServiceChangeHandler(ctx, es, req),
		})

		r.Logger.Info("Public Service added to informer", zap.String("es", req.String()),
			zap.String("public service", es.Spec.Service),
		)
	})

	return nil
}

func (r *ElastiServiceReconciler) checkIfCRDIsDeleted(ctx context.Context, es *v1alpha1.ElastiService, req ctrl.Request) error {
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
	}
	return nil
}

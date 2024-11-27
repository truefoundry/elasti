package controller

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"truefoundry/elasti/operator/api/v1alpha1"
	"truefoundry/elasti/operator/internal/crdDirectory"
	"truefoundry/elasti/operator/internal/informer"
	"truefoundry/elasti/operator/internal/prom"

	"github.com/truefoundry/elasti/pkg/k8shelper"
	"github.com/truefoundry/elasti/pkg/utils"
	"github.com/truefoundry/elasti/pkg/values"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ElastiServiceReconciler) getCRD(ctx context.Context, crdNamespacedName types.NamespacedName) (*v1alpha1.ElastiService, error) {
	es := &v1alpha1.ElastiService{}
	if err := r.Get(ctx, crdNamespacedName, es); err != nil {
		return nil, fmt.Errorf("failed to get ElastiService: %w", err)
	}
	return es, nil
}

func (r *ElastiServiceReconciler) updateCRDStatus(ctx context.Context, crdNamespacedName types.NamespacedName, mode string) (err error) {
	defer func() {
		errStr := values.Success
		if err != nil {
			errStr = err.Error()
		}
		prom.CRDUpdateCounter.WithLabelValues(crdNamespacedName.String(), mode, errStr).Inc()
		var modeGauge float64
		modeGauge = 0
		if mode == values.ProxyMode {
			modeGauge = 1
		}
		prom.ModeGauge.WithLabelValues(crdNamespacedName.String()).Set(modeGauge)
	}()
	es := &v1alpha1.ElastiService{}
	if err = r.Client.Get(ctx, crdNamespacedName, es); err != nil {
		r.Logger.Error("Failed to get ElastiService for status update", zap.String("es", crdNamespacedName.String()), zap.Error(err))
		return fmt.Errorf("failed to get elastiService for status update")
	}
	es.Status.LastReconciledTime = metav1.Now()
	es.Status.Mode = mode

	requestInQueueGauge := prom.RequestInQueueGauge.WithLabelValues(es.Namespace, es.Spec.Service)
	if requestInQueueGauge == nil || mode == values.ServeMode {
		prom.RequestInQueueGauge.WithLabelValues(es.Namespace, es.Spec.Service).Set(0)
	}

	if err = r.Status().Update(ctx, es); err != nil {
		r.Logger.Error("Failed to update status", zap.String("es", crdNamespacedName.String()), zap.Error(err))
		return fmt.Errorf("failed to update CRD status")
	}

	r.Logger.Info("CRD Status updated successfully")
	return nil
}

func (r *ElastiServiceReconciler) addCRDFinalizer(ctx context.Context, es *v1alpha1.ElastiService) error {
	// If the CRD does not contain the finalizer, we add the finalizer
	if !controllerutil.ContainsFinalizer(es, v1alpha1.ElastiServiceFinalizer) {
		controllerutil.AddFinalizer(es, v1alpha1.ElastiServiceFinalizer)
		if err := r.Update(ctx, es); err != nil {
			return fmt.Errorf("failed to add finalizer: %w", err)
		}
	}
	return nil
}

// finalizeCRD reset changes made for the CRD
func (r *ElastiServiceReconciler) finalizeCRD(ctx context.Context, es *v1alpha1.ElastiService, req ctrl.Request) error {
	r.Logger.Info("ElastiService is being deleted", zap.String("name", es.Name), zap.Any("deletionTimestamp", es.ObjectMeta.DeletionTimestamp))
	var wg sync.WaitGroup
	wg.Add(3)
	// Stop all active informers related to this CRD in background
	go func() {
		defer wg.Done()
		r.Informer.StopForCRD(req.Name)
		r.Logger.Info("[Done] Informer stopped for CRD", zap.String("es", req.String()))
		// Reset the informer start mutex, so if the ElastiService is recreated, we will need to reset the informer
		r.resetMutexForInformer(r.getMutexKeyForTargetRef(req))
		r.resetMutexForInformer(r.getMutexKeyForPublicSVC(req))
		r.Logger.Info("[Done] Informer mutex reset for ScaleTargetRef and PublicSVC", zap.String("es", req.String()))
	}()
	targetNamespacedName := types.NamespacedName{
		Name:      es.Spec.Service,
		Namespace: es.Namespace,
	}
	var err1, err2 error
	go func() {
		defer wg.Done()
		// Delete EndpointSlice to resolver
		err1 = r.deleteEndpointsliceToResolver(ctx, targetNamespacedName)
		if err1 == nil {
			r.Logger.Info("[Done] EndpointSlice to resolver deleted", zap.String("service", targetNamespacedName.String()))
		}
	}()
	go func() {
		defer wg.Done()
		// Delete private service
		err2 = r.deletePrivateService(ctx, targetNamespacedName)
		if err2 == nil {
			r.Logger.Info("[Done] Private service deleted", zap.String("service", targetNamespacedName.String()))
		}
	}()
	wg.Wait()
	// Remove CRD details from service directory
	crdDirectory.CRDDirectory.RemoveCRD(es.Spec.Service)
	r.Logger.Info("[Done] CRD removed from service directory", zap.String("es", req.String()))

	if err1 != nil || err2 != nil {
		return fmt.Errorf("failed to finalize CRD. \n Error 1: %w \n Error 2: %w", err1, err2)
	}
	r.Logger.Info("[SERVE MODE ENABLED]")
	return nil
}

// watchScaleTargetRef checks if the ScaleTargetRef has changed, and if it has, stops the informer for the old ScaleTargetRef
// Start the new informer for the new ScaleTargetRef
func (r *ElastiServiceReconciler) watchScaleTargetRef(ctx context.Context, es *v1alpha1.ElastiService, req ctrl.Request) error {
	if es.Spec.ScaleTargetRef.Name == "" ||
		es.Spec.ScaleTargetRef.Kind == "" ||
		es.Spec.ScaleTargetRef.APIVersion == "" {
		return fmt.Errorf("scaleTargetRef is incomplete: %w", k8shelper.ErrNoScaleTargetFound)
	}

	crd, found := crdDirectory.CRDDirectory.GetCRD(es.Spec.Service)
	if found {
		if es.Spec.ScaleTargetRef.Name != crd.Spec.ScaleTargetRef.Name ||
			es.Spec.ScaleTargetRef.Kind != crd.Spec.ScaleTargetRef.Kind ||
			es.Spec.ScaleTargetRef.APIVersion != crd.Spec.ScaleTargetRef.APIVersion {
			r.Logger.Debug("ScaleTargetRef has changed, stopping previous informer.", zap.String("es", req.String()), zap.Any("scaleTargetRef", es.Spec.ScaleTargetRef))
			key := r.Informer.GetKey(informer.KeyParams{
				Namespace:    req.Namespace,
				CRDName:      req.Name,
				ResourceName: crd.Spec.ScaleTargetRef.Name,
				Resource:     strings.ToLower(crd.Spec.ScaleTargetRef.Kind),
			})
			err := r.Informer.StopInformer(key)
			if err != nil {
				r.Logger.Error("Failed to stop informer for old scaleTargetRef", zap.String("es", req.String()), zap.Any("scaleTargetRef", es.Spec.ScaleTargetRef), zap.Error(err))
			}
			r.Logger.Debug("Resetting mutex for old scaleTargetRef informer", zap.Any("scaleTargetRef", es.Spec.ScaleTargetRef))
			r.resetMutexForInformer(r.getMutexKeyForTargetRef(req))
		}
	}

	var informerErr error
	r.getMutexForInformerStart(r.getMutexKeyForTargetRef(req)).Do(func() {
		targetGroup, targetVersion, err := utils.ParseAPIVersion(es.Spec.ScaleTargetRef.APIVersion)
		if err != nil {
			informerErr = fmt.Errorf("failed to parse API version: %w", err)
			return
		}
		if err := r.Informer.Add(&informer.RequestWatch{
			Req:               req,
			ResourceName:      es.Spec.ScaleTargetRef.Name,
			ResourceNamespace: req.Namespace,
			GroupVersionResource: &schema.GroupVersionResource{
				Group:    targetGroup,
				Version:  targetVersion,
				Resource: strings.ToLower(es.Spec.ScaleTargetRef.Kind),
			},
			Handlers: r.getScaleTargetRefChangeHandler(ctx, es, req),
		}); err != nil {
			informerErr = fmt.Errorf("failed to add scaledTargetRef Informer: %w", err)
			return
		}
	})
	if informerErr != nil {
		return informerErr
	}
	return nil
}

// watchPublicService checks if the Public Service has changed, and makes sure it's not null
func (r *ElastiServiceReconciler) watchPublicService(ctx context.Context, es *v1alpha1.ElastiService, req ctrl.Request) error {
	if es.Spec.Service == "" {
		return fmt.Errorf("null value for public service: %w", k8shelper.ErrNoPublicServiceFound)
	}
	var informerErr error
	r.getMutexForInformerStart(r.getMutexKeyForPublicSVC(req)).Do(func() {
		if err := r.Informer.Add(&informer.RequestWatch{
			Req:                  req,
			ResourceName:         es.Spec.Service,
			ResourceNamespace:    es.Namespace,
			GroupVersionResource: &values.ServiceGVR,
			Handlers:             r.getPublicServiceChangeHandler(ctx, es, req),
		}); err != nil {
			informerErr = fmt.Errorf("failed to add public service Informer: %w", err)
			return
		}
	})
	if informerErr != nil {
		return informerErr
	}
	return nil
}

func (r *ElastiServiceReconciler) finalizeCRDIfDeleted(ctx context.Context, es *v1alpha1.ElastiService, req ctrl.Request) (delete bool, err error) {
	// If the ElastiService is being deleted, we need to clean up the resources
	if !es.ObjectMeta.DeletionTimestamp.IsZero() {
		defer func() {
			e := values.Success
			if err != nil {
				e = err.Error()
			}
			prom.CRDFinalizerCounter.WithLabelValues(req.String(), e).Inc()
		}()
		if controllerutil.ContainsFinalizer(es, v1alpha1.ElastiServiceFinalizer) {
			// If CRD contains finalizer, we call the finaizer function and remove the finalizer post that
			if err = r.finalizeCRD(ctx, es, req); err != nil {
				return true, err
			}
			controllerutil.RemoveFinalizer(es, v1alpha1.ElastiServiceFinalizer)
			if err = r.Update(ctx, es); err != nil {
				return true, fmt.Errorf("failed to remove finalizer: %w", err)
			}
		}
		return true, nil
	}
	return false, nil
}

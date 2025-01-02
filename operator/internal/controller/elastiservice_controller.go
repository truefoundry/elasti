package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"

	"truefoundry/elasti/operator/internal/crddirectory"
	"truefoundry/elasti/operator/internal/informer"
	"truefoundry/elasti/operator/internal/prom"

	"github.com/truefoundry/elasti/pkg/values"
	"k8s.io/apimachinery/pkg/api/errors"
	kRuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"truefoundry/elasti/operator/api/v1alpha1"

	"go.uber.org/zap"
)

type (
	SwitchModeFunc          func(ctx context.Context, req ctrl.Request, mode string) (res ctrl.Result, err error)
	ElastiServiceReconciler struct {
		client.Client
		Scheme             *kRuntime.Scheme
		Logger             *zap.Logger
		InformerManager    *informer.Manager
		SwitchModeLocks    sync.Map
		InformerStartLocks sync.Map
		ReconcileLocks     sync.Map
	}
)

const (

	// These are resolver details, ideally in future we can move this to a configmap, or find a better way to serve this
	resolverNamespace      = "elasti"
	resolverDeploymentName = "elasti-resolver"
	resolverServiceName    = "elasti-resolver-service"
	resolverPort           = 8012
)

//+kubebuilder:rbac:groups=elasti.truefoundry.com,resources=elastiservices,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=elasti.truefoundry.com,resources=elastiservices/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=elasti.truefoundry.com,resources=elastiservices/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ElastiService object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *ElastiServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	r.Logger.Debug("- In Reconcile", zap.String("es", req.NamespacedName.String()))
	mutex := r.getMutexForReconcile(req.NamespacedName.String())
	mutex.Lock()
	defer r.Logger.Debug("- Out of Reconcile", zap.String("es", req.NamespacedName.String()))
	defer mutex.Unlock()
	startTime := time.Now()

	defer func() {
		e := values.Success
		if err != nil {
			e = err.Error()
			sentry.CaptureException(err)
		}
		duration := time.Since(startTime).Seconds()
		prom.CRDReconcileHistogram.WithLabelValues(req.String(), e).Observe(duration)
	}()

	es, esErr := r.getCRD(ctx, req.NamespacedName)
	if esErr != nil {
		if errors.IsNotFound(esErr) {
			r.Logger.Error("ElastiService not found.", zap.String("es", req.String()))
			return res, nil
		}
		r.Logger.Error("Failed to get ElastiService in Reconcile", zap.String("es", req.String()), zap.Error(esErr))
		sentry.CaptureException(esErr)
		return res, esErr
	}

	// If the ElastiService is being deleted, we need to clean up the resources
	if isDeleted, err := r.finalizeCRDIfDeleted(ctx, es, req); err != nil {
		r.Logger.Error("Failed to check if CRD is deleted", zap.String("es", req.String()), zap.Error(err))
		sentry.CaptureException(err)
		return res, err
	} else if isDeleted {
		r.Logger.Info("[CRD is deleted successfully]", zap.String("es", req.String()))
		return res, nil
	}

	// We also check if the CRD has finalizer, and if not, we add the finalizer
	if err := r.addCRDFinalizer(ctx, es); err != nil {
		r.Logger.Error("Failed to finalize CRD", zap.String("es", req.String()), zap.Error(err))
		sentry.CaptureException(err)
		return res, err
	}
	r.Logger.Info("Finalizer added to CRD", zap.String("es", req.String()))

	// Add watch for public service, so when the public service is modified, we can update the private service
	if err := r.watchScaleTargetRef(ctx, es, req); err != nil {
		r.Logger.Error("Failed to add watch for ScaleTargetRef", zap.String("es", req.String()), zap.Any("scaleTargetRef", es.Spec.ScaleTargetRef), zap.Error(err))
		sentry.CaptureException(err)
		return res, err
	}
	r.Logger.Info("Watch added for ScaleTargetRef", zap.String("es", req.String()), zap.Any("scaleTargetRef", es.Spec.ScaleTargetRef))

	// We add the CRD details to service directory, so when elasti server received a request,
	// we can find the right resource to scale up
	crddirectory.CRDDirectory.AddCRD(es.Spec.Service, &crddirectory.CRDDetails{
		CRDName: es.Name,
		Spec:    es.Spec,
	})
	r.Logger.Info("CRD added to service directory", zap.String("es", req.String()), zap.String("service", es.Spec.Service))
	return res, nil
}

func (r *ElastiServiceReconciler) ForceReconcile(ctx context.Context, serviceName, namespace string) error {
	r.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      serviceName,
			Namespace: namespace,
		},
	})
	return nil
}

func (r *ElastiServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ElastiService{}).
		Complete(r)
	if err != nil {
		return fmt.Errorf("SetupWithManager: %w", err)
	}
	return nil
}

func (r *ElastiServiceReconciler) getMutexForReconcile(key string) *sync.Mutex {
	l, _ := r.ReconcileLocks.LoadOrStore(key, &sync.Mutex{})
	return l.(*sync.Mutex)
}

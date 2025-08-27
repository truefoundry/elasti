package controller

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/truefoundry/elasti/pkg/scaling"
	"k8s.io/apimachinery/pkg/types"

	"truefoundry/elasti/operator/internal/crddirectory"
	"truefoundry/elasti/operator/internal/informer"
	"truefoundry/elasti/operator/internal/prom"

	"github.com/truefoundry/elasti/pkg/values"
	"k8s.io/apimachinery/pkg/api/errors"
	kRuntime "k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"truefoundry/elasti/operator/api/v1alpha1"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type (
	SwitchModeFunc          func(ctx context.Context, req ctrl.Request, mode string) (res ctrl.Result, err error)
	ElastiServiceReconciler struct {
		client.Client
		Scheme             *kRuntime.Scheme
		Logger             *zap.Logger
		InformerManager    *informer.Manager
		SwitchModeLocks    sync.Map
		ScaleHandler       *scaling.ScaleHandler
		InformerStartLocks sync.Map
		ReconcileLocks     sync.Map
	}
)

const (

	// These are resolver details, ideally in future we can move this to a configmap, or find a better way to serve this
	// TODO: Move this to configmap
	resolverDeploymentName = "elasti-resolver"
	resolverServiceName    = "elasti-resolver-service"
	resolverPort           = 8012
)

var (
	// TODO: move this to configmap as const above
	resolverNamespace = os.Getenv("ELASTI_POD_NAMESPACE")
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
			r.Logger.Error("Error reconciling ElastiService.", zap.String("es", req.NamespacedName.String()), zap.Error(err))
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
		return res, esErr
	}

	// If the ElastiService is being deleted, we need to clean up the resources
	if isDeleted, err := r.finalizeCRDIfDeleted(ctx, es, req); err != nil {
		r.Logger.Error("Failed to check if CRD is deleted", zap.String("es", req.String()), zap.Error(err))
		return res, err
	} else if isDeleted {
		r.Logger.Info("[CRD is deleted successfully]", zap.String("es", req.String()))
		return res, nil
	}

	// We also check if the CRD has finalizer, and if not, we add the finalizer
	if err := r.addCRDFinalizer(ctx, es); err != nil {
		r.Logger.Error("Failed to finalize CRD", zap.String("es", req.String()), zap.Error(err))
		return res, err
	}
	r.Logger.Info("Finalizer added to CRD", zap.String("es", req.String()))

	// Add watch for public service, so when the public service is modified, we can update the private service
	if err := r.watchScaleTargetRef(ctx, es, req); err != nil {
		r.Logger.Error("Failed to add watch for ScaleTargetRef", zap.String("es", req.String()), zap.Any("scaleTargetRef", es.Spec.ScaleTargetRef), zap.Error(err))
		return res, err
	}
	r.Logger.Info("Watch added for ScaleTargetRef", zap.String("es", req.String()), zap.Any("scaleTargetRef", es.Spec.ScaleTargetRef))

	// We add the CRD details to service directory, so when elasti server received a request,
	// we can find the right resource to scale up
	svcNamespacedName := types.NamespacedName{Name: es.Spec.Service, Namespace: es.Namespace}
	crddirectory.AddCRD(svcNamespacedName.String(), &crddirectory.CRDDetails{
		CRDName: es.Name,
		Spec:    es.Spec,
		Status:  es.Status,
	})
	r.Logger.Info("CRD added to service directory", zap.String("es", req.String()), zap.String("service", es.Spec.Service))
	return res, nil
}

func (r *ElastiServiceReconciler) SetupWithManager(mgr ctrl.Manager, watchNamespace string) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ElastiService{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			es, ok := obj.(*v1alpha1.ElastiService)
			if !ok {
				return false
			}
			if watchNamespace == metav1.NamespaceAll || es.Namespace == watchNamespace {
				return true
			}
			return false
		})).
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

func (r *ElastiServiceReconciler) Initialize(ctx context.Context, watchNamespace string) error {
	if err := r.reconcileExistingCRDs(ctx, watchNamespace); err != nil {
		return fmt.Errorf("failed to reconcile existing CRDs: %w", err)
	}
	if err := r.InformerManager.InitializeResolverInformer(r.getResolverChangeHandler(ctx)); err != nil {
		return fmt.Errorf("failed to initialize resolver informer: %w", err)
	}
	r.ScaleHandler.StartScaleDownWatcher(ctx)
	return nil
}

func (r *ElastiServiceReconciler) reconcileExistingCRDs(ctx context.Context, watchNamespace string) error {
	crdList := &v1alpha1.ElastiServiceList{}
	if err := r.List(ctx, crdList, client.InNamespace(watchNamespace)); err != nil {
		return fmt.Errorf("failed to list ElastiServices: %w", err)
	}
	count := 0

	for _, es := range crdList.Items {
		// Skip if being deleted
		if !es.ObjectMeta.DeletionTimestamp.IsZero() {
			r.Logger.Debug("Skipping ElastiService because it is being deleted", zap.String("name", es.Name), zap.String("namespace", es.Namespace))
			continue
		}

		req := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      es.Name,
				Namespace: es.Namespace,
			},
		}

		if _, err := r.Reconcile(ctx, req); err != nil {
			r.Logger.Error(
				"Failed to reconcile existing ElastiService",
				zap.String("name", es.Name),
				zap.String("namespace", es.Namespace),
				zap.Error(err),
			)
			continue
		}
		count++
		r.Logger.Info(
			"Reconciled existing ElastiService",
			zap.String("name", es.Name),
			zap.String("namespace", es.Namespace),
		)
	}

	r.Logger.Info("Successfully reconciled all existing ElastiServices", zap.Int("count", count))

	return nil
}

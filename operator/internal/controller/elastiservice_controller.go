package controller

import (
	"context"
	"sync"

	"truefoundry.io/elasti/internal/crdDirectory"
	"truefoundry.io/elasti/internal/informer"

	"k8s.io/apimachinery/pkg/api/errors"
	kRuntime "k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"truefoundry.io/elasti/api/v1alpha1"

	"go.uber.org/zap"
)

type (
	SwitchModeFunc          func(ctx context.Context, req ctrl.Request, mode string) (res ctrl.Result, err error)
	ElastiServiceReconciler struct {
		client.Client
		Scheme             *kRuntime.Scheme
		Logger             *zap.Logger
		Informer           *informer.Manager
		SwitchModeLocks    sync.Map
		InformerStartLocks sync.Map
		ReconcileLocks     sync.Map
	}
)

const (

	// These are resolver details, ideally in future we can move this to a configmap, or find a better way to serve this
	resolverNamespace      = "elasti"
	resolverDeploymentName = "elasti-resolver"
	resolverServiceName    = "resolver-service"
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

	es, esErr := r.getCRD(ctx, req.NamespacedName)
	if esErr != nil {
		if errors.IsNotFound(esErr) {
			r.Logger.Info("ElastiService not found", zap.String("es", req.String()))
			return res, nil
		}
		r.Logger.Error("Failed to get ElastiService in Reconcile", zap.String("es", req.String()), zap.Error(esErr))
		return res, esErr
	}

	// If the ElastiService is being deleted, we need to clean up the resources
	if err := r.checkIfCRDIsDeleted(ctx, es, req); err != nil {
		r.Logger.Error("Failed to check if CRD is deleted", zap.String("es", req.String()), zap.Error(err))
		return res, err
	}

	// We also check if the CRD has finalizer, and if not, we add the finalizer
	if err := r.checkAndAddCRDFinalizer(ctx, es, req); err != nil {
		r.Logger.Error("Failed to finalize CRD", zap.String("es", req.String()), zap.Error(err))
		return res, err
	}

	// Check if ScaleTargetRef is present, and has not changed from the values in CRDDirectory
	if err := r.checkChangesInScaleTargetRef(ctx, es, req); err != nil {
		r.Logger.Error("Failed to check changes in ScaleTargetRef", zap.String("es", req.String()), zap.Error(err))
		return res, err
	}

	// Check if Public Service is present, and has not changed from the values in CRDDirectory
	if err := r.checkChangesInPublicService(ctx, es, req); err != nil {
		r.Logger.Error("Failed to check changes in public service", zap.String("es", req.String()), zap.Error(err))
		return res, err
	}

	// We add the CRD details to service directory, so when elasti server received a request,
	// we can find the right resource to scale up
	crdDirectory.CRDDirectory.AddCRD(es.Spec.Service, &crdDirectory.CRDDetails{
		CRDName: es.Name,
		Spec:    es.Spec,
	})
	return res, nil
}

func (r *ElastiServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ElastiService{}).
		Complete(r)
}

func (r *ElastiServiceReconciler) getMutexForReconcile(key string) *sync.Mutex {
	l, _ := r.ReconcileLocks.LoadOrStore(key, &sync.Mutex{})
	return l.(*sync.Mutex)
}

package controller

import (
	"context"
	"sync"

	"truefoundry.io/elasti/internal/crdDirectory"
	"truefoundry.io/elasti/internal/informer"

	"k8s.io/apimachinery/pkg/api/errors"
	kRuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"truefoundry.io/elasti/api/v1alpha1"

	"runtime"

	"go.uber.org/zap"
)

type (
	RunReconcileFunc        func(ctx context.Context, req ctrl.Request, mode string) (res ctrl.Result, err error)
	ElastiServiceReconciler struct {
		client.Client
		Scheme            *kRuntime.Scheme
		Logger            *zap.Logger
		Informer          *informer.Manager
		RunReconcileLocks sync.Map
		WatcherStartLock  sync.Map
	}
)

const (
	ServeMode = "serve"
	ProxyMode = "proxy"
	NullMode  = ""

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
	defer func() {
		if rErr := recover(); rErr != nil {
			r.Logger.Error("Recovered from panic", zap.Any("recovered", rErr))
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			r.Logger.Error("Panic stack trace", zap.ByteString("stacktrace", buf[:n]))
		}
	}()
	// First we get the ElastiService object
	// No mutex is taken for this, as we are not modifying the object, but if we face issues in future, we can add a mutex
	es, esErr := r.getCRD(ctx, req.NamespacedName)
	if esErr != nil {
		if errors.IsNotFound(esErr) {
			r.Logger.Info("ElastiService not found", zap.String("es", req.String()))
			return res, nil
		}
		r.Logger.Error("Failed to get ElastiService in Reconcile", zap.String("es", req.String()), zap.Error(esErr))
		return res, esErr
	}
	// We add the CRD details to service directory, so when elasti server received a request,
	// we can find the right resource to scale up
	crdDirectory.CRDDirectory.AddCRD(es.Spec.Service, &crdDirectory.CRDDetails{
		CRDName:        es.Name,
		DeploymentName: es.Spec.DeploymentName,
	})

	// If the ElastiService is being deleted, we need to clean up the resources
	if !es.ObjectMeta.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(es, v1alpha1.ElastiServiceFinalizer) {
			// If CRD contains finalizer, we call the finaizer function and remove the finalizer post that
			if err := r.finalizeCRD(ctx, es, req); err != nil {
				r.Logger.Error("Failed to enable serve mode", zap.String("es", req.String()), zap.Error(err))
				return res, err
			}
			controllerutil.RemoveFinalizer(es, v1alpha1.ElastiServiceFinalizer)
			if err := r.Update(ctx, es); err != nil {
				return res, err
			}
		}
		return res, nil
	}

	// We check if the CRD is being deleted, and if it is, we clean up the resources
	// We also check if the CRD has finalizer, and if not, we add the finalizer
	if err := r.checkFinalizerCRD(ctx, es, req); err != nil {
		r.Logger.Error("Failed to finalize CRD", zap.String("es", req.String()), zap.Error(err))
		return res, err
	}

	// We need to start the informer only once per CRD. This is to avoid multiple informers for the same CRD
	// We reset mutex if crd is deleted, so it can be used again if the same CRD is reapplied
	r.getMutexForInformerStart(req.NamespacedName.String()).Do(func() {
		// Watch for changes in target deployment
		go r.Informer.AddDeploymentWatch(req, es.Spec.DeploymentName, req.Namespace, r.getTargetDeploymentChangeHandler(ctx, es, req))
		// Watch for changes in activator deployment
		go r.Informer.AddDeploymentWatch(req, resolverDeploymentName, resolverNamespace, r.getResolverChangeHandler(ctx, es, req))
	})

	deploymentNamespacedName := types.NamespacedName{
		Name:      es.Spec.DeploymentName,
		Namespace: req.Namespace,
	}
	mode, err := r.getModeFromDeployment(ctx, deploymentNamespacedName)
	if err != nil {
		r.Logger.Error("Failed to get mode from deployment", zap.String("es", req.NamespacedName.String()), zap.Error(err))
		return res, err
	}
	// Run the reconcile function for any change in CRD
	return r.runReconcile(ctx, req, mode)
}

func (r *ElastiServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ElastiService{}).
		Complete(r)
}

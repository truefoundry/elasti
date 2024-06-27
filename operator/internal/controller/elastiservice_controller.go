package controller

import (
	"context"
	"strings"
	"sync"

	"github.com/truefoundry/elasti/pkg/utils"
	"truefoundry.io/elasti/internal/crdDirectory"
	"truefoundry.io/elasti/internal/informer"

	"k8s.io/apimachinery/pkg/api/errors"
	kRuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"truefoundry.io/elasti/api/v1alpha1"

	"runtime"

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
	defer func() {
		if rErr := recover(); rErr != nil {
			r.Logger.Error("Recovered from panic", zap.Any("recovered", rErr))
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			r.Logger.Error("Panic stack trace", zap.ByteString("stacktrace", buf[:n]))
		}
	}()

	r.Logger.Debug("- In Reconcile", zap.String("es", req.NamespacedName.String()))
	mutex := r.getMutexForReconcile(req.NamespacedName.String())
	mutex.Lock()
	defer r.Logger.Debug("- Out of Reconcile", zap.String("es", req.NamespacedName.String()))
	defer mutex.Unlock()

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

	// We also check if the CRD has finalizer, and if not, we add the finalizer
	if err := r.checkFinalizerCRD(ctx, es, req); err != nil {
		r.Logger.Error("Failed to finalize CRD", zap.String("es", req.String()), zap.Error(err))
		return res, err
	}

	// We add the CRD details to service directory, so when elasti server received a request,
	// we can find the right resource to scale up
	crdDirectory.CRDDirectory.AddCRD(es.Spec.Service, &crdDirectory.CRDDetails{
		CRDName: es.Name,
		Spec:    es.Spec,
	})

	// We need to start the informer only once per CRD. This is to avoid multiple informers for the same CRD
	// We reset mutex if crd is deleted, so it can be used again if the same CRD is reapplied
	r.getMutexForInformerStart(req.NamespacedName.String()).Do(func() {
		targetGroup, targetVersion, err := utils.ParseAPIVersion(es.Spec.ScaleTargetRef.APIVersion)
		if err != nil {
			r.Logger.Error("Failed to parse API version", zap.String("APIVersion", es.Spec.ScaleTargetRef.APIVersion), zap.Error(err))
			return
		}

		// Watch for changes in ScaleTargetRef
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

		// Watch for changes in public service
		r.Informer.Add(&informer.RequestWatch{
			Req:               req,
			ResourceName:      es.Spec.Service,
			ResourceNamespace: es.Namespace,
			GroupVersionResource: &schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "services",
			},
			Handlers: r.getPublicServiceChangeHandler(ctx, es, req),
		})

		r.Logger.Info("ScaleTargetRef and Public Service added to informer", zap.String("es", req.String()),
			zap.String("scaleTargetRef", es.Spec.ScaleTargetRef.Name),
			zap.String("public service", es.Spec.Service),
		)
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

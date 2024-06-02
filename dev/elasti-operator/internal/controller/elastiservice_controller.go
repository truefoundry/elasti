package controller

import (
	"context"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"truefoundry.io/elasti/api/v1alpha1"

	"go.uber.org/zap"
)

type (
	RunReconcileFunc        func(ctx context.Context, req ctrl.Request, mode string) (res ctrl.Result, err error)
	ElastiServiceReconciler struct {
		client.Client
		Scheme  *runtime.Scheme
		Logger  *zap.Logger
		Watcher *WatcherType
	}
)

const (
	ServeMode = "serve"
	ProxyMode = "proxy"
	NullMode  = ""
)

var locks sync.Map

func getMutexForRequest(key string) *sync.Mutex {
	l, _ := locks.LoadOrStore(key, &sync.Mutex{})
	return l.(*sync.Mutex)
}

//+kubebuilder:rbac:groups=elasti.truefoundry.io,resources=elastiservices,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=elasti.truefoundry.io,resources=elastiservices/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=elasti.truefoundry.io,resources=elastiservices/finalizers,verbs=update

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
	es, esErr := r.GetES(ctx, req.NamespacedName)
	if esErr != nil {
		if errors.IsNotFound(esErr) {
			r.Logger.Info("ElastiService not found", zap.String("name", req.Name))
			return res, nil
		}
		r.Logger.Error("Failed to get ElastiService in Reconcile", zap.Error(esErr))
		return res, esErr
	}
	if !es.ObjectMeta.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(es, v1alpha1.ElastiServiceFinalizer) {
			r.Logger.Info("ElastiService is being deleted", zap.String("name", es.Name), zap.Any("deletionTimestamp", es.ObjectMeta.DeletionTimestamp))
			go r.Watcher.StopDeplymentWatch(es.Spec.DeploymentName)
			if err = r.EnableServeMode(ctx, es); err != nil {
				r.Logger.Error("Failed to server mode", zap.Error(err))
				return res, err
			}
			r.Logger.Info("Serve mode enabled")
			controllerutil.RemoveFinalizer(es, v1alpha1.ElastiServiceFinalizer)
			if err := r.Update(ctx, es); err != nil {
				return res, err
			}
		}
		return res, nil
	}
	if !controllerutil.ContainsFinalizer(es, v1alpha1.ElastiServiceFinalizer) {
		controllerutil.AddFinalizer(es, v1alpha1.ElastiServiceFinalizer)
		if err = r.Update(ctx, es); err != nil {
			r.Logger.Error("Failed to add finalizer", zap.Error(err))
			return res, err
		} else {
			r.Logger.Info("Finalizer added")
		}
	}

	go r.Watcher.AddAndRunDeploymentWatch(es.Spec.DeploymentName, req, r.RunReconcile)
	return r.RunReconcile(ctx, req, NullMode)
}

func (r *ElastiServiceReconciler) RunReconcile(ctx context.Context, req ctrl.Request, mode string) (res ctrl.Result, err error) {
	mutex := getMutexForRequest(req.NamespacedName.String())
	mutex.Lock()
	r.Logger.Debug("In RunReconcile", zap.String("key", req.NamespacedName.String()))
	defer r.Logger.Debug("Out of RunReconcile", zap.String("key", req.NamespacedName.String()))
	defer mutex.Unlock()
	defer r.UpdateESStatus(ctx, req.NamespacedName, mode)
	es, err := r.GetES(ctx, req.NamespacedName)
	if mode != ProxyMode && mode != ServeMode {
		nam := types.NamespacedName{
			Name:      es.Spec.DeploymentName,
			Namespace: req.Namespace,
		}
		mode, err = r.GetModeFromDeployment(ctx, nam)
		if err != nil {
			r.Logger.Error("Failed to get mode from deployment", zap.Error(err))
			return res, err
		}
	}

	switch mode {
	case ServeMode:
		if err = r.EnableServeMode(ctx, es); err != nil {
			r.Logger.Error("Failed to enable serve mode", zap.Error(err))
			return res, err
		}
		r.Logger.Info("Serve mode enabled")
	case ProxyMode:
		if err = r.EnableProxyMode(ctx, es); err != nil {
			r.Logger.Error("Failed to enable proxy mode", zap.Error(err))
			return res, err
		}
		r.Logger.Debug("Proxy mode enabled")
	}

	return res, nil
}

func (r *ElastiServiceReconciler) EnableProxyMode(ctx context.Context, es *v1alpha1.ElastiService) error {
	targetNamespacedName := types.NamespacedName{
		Name:      es.Spec.Service,
		Namespace: es.Namespace,
	}
	targetSVC := &v1.Service{}
	if err := r.Get(ctx, targetNamespacedName, targetSVC); err != nil {
		r.Logger.Error("Failed to get target service", zap.Error(err))
		return err
	}
	_, err := r.CheckAndCreatePrivateService(ctx, targetSVC, es)
	if err != nil {
		return err
	}
	if err = r.CreateOrupdateEndpointsliceToActivator(ctx, targetSVC); err != nil {
		return err
	}
	return nil
}

func (r *ElastiServiceReconciler) EnableServeMode(ctx context.Context, es *v1alpha1.ElastiService) error {
	targetNamespacedName := types.NamespacedName{
		Name:      es.Spec.Service,
		Namespace: es.Namespace,
	}
	if err := r.DeleteEndpointsliceToActivator(ctx, targetNamespacedName); err != nil {
		return err
	}
	if err := r.DeletePrivateService(ctx, targetNamespacedName); err != nil {
		return err
	}
	return nil
}

func (r *ElastiServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	go r.StartElastiServer()
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ElastiService{}).
		Complete(r)
}

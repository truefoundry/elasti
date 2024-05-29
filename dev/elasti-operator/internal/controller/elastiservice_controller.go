package controller

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"truefoundry.io/elasti/api/v1alpha1"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type (
	RunReconcileFunc        func(ctx context.Context, req ctrl.Request, es *v1alpha1.ElastiService, mode string) (res ctrl.Result, err error)
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

var esLock = &sync.Mutex{}

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
	es := &v1alpha1.ElastiService{}
	esLock.Lock()
	if err = r.Client.Get(ctx, req.NamespacedName, es); err != nil {
		esLock.Unlock()
		return res, err
	}
	esLock.Unlock()
	go r.Watcher.AddAndRunDeploymentWatch(es.Spec.DeploymentName, req.Namespace, es, r.RunReconcile)
	return r.RunReconcile(ctx, req, es, es.Status.Mode)
}

func (r *ElastiServiceReconciler) RunReconcile(ctx context.Context, req ctrl.Request, es *v1alpha1.ElastiService, mode string) (res ctrl.Result, err error) {
	if mode == NullMode {
		r.Logger.Info("No mode specified")
		mode = ServeMode
	}

	if mode == ProxyMode {
		r.Logger.Info("Enabling proxy mode")
		if err = r.EnableProxyMode(ctx, es); err != nil {
			r.Logger.Error("Failed to enable proxy mode", zap.Error(err))
			return res, err
		}
		r.Logger.Info("Proxy mode enabled")
	} else if mode == ServeMode {
		r.Logger.Info("Enabling Serve mode")
		if err = r.serveMode(ctx, es); err != nil {
			r.Logger.Error("Failed to serve mode", zap.Error(err))
			return res, err
		}
		r.Logger.Info("Serve mode enabled")
	} else {
		r.Logger.Error("No mode or incorrect mode specified")
		return res, nil
	}

	esLock.Lock()
	defer esLock.Unlock()
	if err = r.Client.Get(ctx, req.NamespacedName, es); err != nil {
		return res, err
	}
	es.Status.LastReconciledTime = metav1.Now()
	es.Status.Mode = mode
	if err := r.Status().Update(ctx, es); err != nil {
		r.Logger.Error("Failed to update status", zap.Error(err))
		return ctrl.Result{}, err
	}

	return res, nil
}

func (r *ElastiServiceReconciler) EnableProxyMode(ctx context.Context, es *v1alpha1.ElastiService) error {
	targetSVC, err := r.getSVC(ctx, es.Spec.Service, es.Namespace)
	if err != nil {
		return err
	}
	r.removeSelector(ctx, targetSVC)
	r.addTargetPort(ctx, targetSVC, 8012)
	if err = r.Update(ctx, targetSVC); err != nil {
		return err
	}
	if err = r.createProxyEndpointSlice(ctx, targetSVC); err != nil {
		return err
	}
	return nil
}

func (r *ElastiServiceReconciler) serveMode(ctx context.Context, es *v1alpha1.ElastiService) error {
	privateSVCName := es.Spec.Service + "-pvt"
	privateSVC, err := r.getSVC(ctx, privateSVCName, es.Namespace)
	if err != nil {
		return err
	}
	if targetSVC, err := r.getSVC(ctx, es.Spec.Service, es.Namespace); err != nil {
		return err
	} else {
		if err = r.checkAndDeleteEendpointslices(ctx, es.Spec.Service); err != nil {
			return err
		}
		if err = r.copySVC(ctx, targetSVC, privateSVC); err != nil {
			return err
		}
		if err = r.Update(ctx, targetSVC); err != nil {
			return err
		}
	}
	return nil
}

func (r *ElastiServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	go r.StartElastiServer()
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ElastiService{}).
		Complete(r)
}

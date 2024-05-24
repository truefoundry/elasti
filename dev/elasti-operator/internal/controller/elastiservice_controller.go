package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"truefoundry.io/elasti/api/v1alpha1"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ElastiServiceReconciler reconciles a ElastiService object
type ElastiServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Logger *zap.Logger
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
	es := &v1alpha1.ElastiService{}
	if err = r.Client.Get(ctx, req.NamespacedName, es); err != nil {
		return res, err
	}
	if es.Spec.Mode == "proxy" {
		r.Logger.Info("Enabling proxy mode")
		if err = r.EnableProxyMode(ctx, es); err != nil {
			r.Logger.Error("Failed to enable proxy mode", zap.Error(err))
			return res, err
		}
		r.Logger.Info("Proxy mode enabled")
	} else {
		r.Logger.Info("Enabling Serve mode")
		if err = r.serveMode(ctx, es); err != nil {
			r.Logger.Error("Failed to serve mode", zap.Error(err))
			return res, err
		}
		r.Logger.Info("Serve mode enabled")
	}
	es.Status.LastReconciledTime = metav1.Now()
	err = r.Status().Update(ctx, es)
	if err != nil {
		r.Logger.Error("Failed to update status", zap.Error(err))
		return ctrl.Result{}, err
	}
	return res, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ElastiServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ElastiService{}).
		Complete(r)
}

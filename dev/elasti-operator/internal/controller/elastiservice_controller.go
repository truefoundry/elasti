package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"truefoundry.io/elasti/api/v1alpha1"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const ServeMode = "serve"
const ProxyMode = "proxy"
const NullMode = ""

var statusLock *sync.Mutex

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
	if es.Status.Mode == NullMode {
		es.Status.Mode = ServeMode
	}

	go Watcher.AddDeploymentWatch(es.Spec.DeploymentName, req.Namespace, es, r.RunReconcile)
	return r.RunReconcile(ctx, req, es, es.Status.Mode)
}

type RunReconcileFunc func(ctx context.Context, req ctrl.Request, es *v1alpha1.ElastiService, mode string) (res ctrl.Result, err error)

func (r *ElastiServiceReconciler) RunReconcile(ctx context.Context, req ctrl.Request, es *v1alpha1.ElastiService, mode string) (res ctrl.Result, err error) {
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
		r.Logger.Info("No mode specified")
		return res, nil
	}

	statusLock.Lock()
	defer statusLock.Unlock()
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

type RequestCount struct {
	Count     int    `json:"count"`
	Svc       string `json:"svc"`
	Namespace string `json:"namespace"`
}

// SetupWithManager sets up the controller with the Manager.
func (r *ElastiServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	statusLock = &sync.Mutex{}
	http.HandleFunc("/request-count", func(w http.ResponseWriter, req *http.Request) {
		ctx := context.Background()
		if req.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}
		var body RequestCount
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()
		r.Logger.Info("Received request", zap.Any("body", body))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "Request received"}`))

		namespace := types.NamespacedName{
			Name:      "target",
			Namespace: body.Namespace,
		}
		r.scaleDeployment(ctx, namespace)
		var ctrlReq ctrl.Request
		ctrlReq.NamespacedName = namespace
		statusLock.Lock()
		defer statusLock.Unlock()
		es := &v1alpha1.ElastiService{}
		if err := r.Client.Get(ctx, namespace, es); err != nil {
			r.Logger.Error("Failed to get ElastiService", zap.Error(err))
			return
		}
		es.Status.Mode = ServeMode
		go r.RunReconcile(ctx, ctrlReq, es, ServeMode)
	})
	go http.ListenAndServe(":8080", nil)

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ElastiService{}).
		Complete(r)
}

func (r *ElastiServiceReconciler) scaleDeployment(ctx context.Context, target types.NamespacedName) error {
	// Fetch the Deployment
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, target, deployment); err != nil {
		if errors.IsNotFound(err) {
			r.Logger.Info("Deployment not found", zap.Any("Deployment", target))
			return nil
		}
		return err
	}

	if *deployment.Spec.Replicas == 0 {
		// Scale up the Deployment by 1
		*deployment.Spec.Replicas = 1
		if err := r.Update(ctx, deployment); err != nil {
			r.Logger.Error("Failed to scale up the Deployment", zap.Error(err))
			return err
		}
	}
	return nil
}

package scaling

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"truefoundry/elasti/operator/api/v1alpha1"

	"github.com/truefoundry/elasti/pkg/scaling/scalers"
	"github.com/truefoundry/elasti/pkg/values"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type ScaleHandler struct {
	kClient        *kubernetes.Clientset
	kDynamicClient *dynamic.DynamicClient

	scaleLocks sync.Map

	logger *zap.Logger
}

// getMutexForScale returns a mutex for scaling based on the input key
func (h *ScaleHandler) getMutexForScale(key string) *sync.Mutex {
	l, _ := h.scaleLocks.LoadOrStore(key, &sync.Mutex{})
	return l.(*sync.Mutex)
}

// NewScaleHandler creates a new instance of the ScaleHandler
func NewScaleHandler(logger *zap.Logger, config *rest.Config) *ScaleHandler {
	kClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Fatal("Error connecting with kubernetes", zap.Error(err))
	}

	kDynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		logger.Fatal("Error connecting with kubernetes", zap.Error(err))
	}

	return &ScaleHandler{
		logger:         logger.Named("ScaleHandler"),
		kClient:        kClient,
		kDynamicClient: kDynamicClient,
	}
}

func (h *ScaleHandler) StartScaleDownWatcher(ctx context.Context) {
	ticker := time.NewTicker(time.Second * 30)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				if err := h.checkAndScaleDown(ctx); err != nil {
					h.logger.Error("failed to run the scale down check", zap.Error(err))
				}
			}
		}
	}()
}

func (h *ScaleHandler) checkAndScaleDown(ctx context.Context) error {
	elastiServiceList, err := h.kDynamicClient.Resource(values.ElastiServiceGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list ElastiServices: %w", err)
	}

	for _, item := range elastiServiceList.Items {
		es := &v1alpha1.ElastiService{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, es); err != nil {
			h.logger.Error("failed to convert unstructured to ElastiService", zap.Error(err))
			continue
		}
		if es.Status.Mode == values.ProxyMode {
			continue
		}

		namespacedName := types.NamespacedName{
			Name:      es.Spec.ScaleTargetRef.Name,
			Namespace: es.Namespace,
		}

		scaleDown, err := h.CheckMetricsToScaleDown(ctx, es)
		if err != nil {
			h.logger.Error("Failed to check metrics to scale down", zap.Error(err), zap.String("namespacedName", namespacedName.String()))
			continue
		}

		if scaleDown {
			if err := h.ScaleTargetToZero(namespacedName, es.Spec.ScaleTargetRef.Kind); err != nil {
				h.logger.Error("Failed to scale target to zero", zap.Error(err), zap.String("namespacedName", namespacedName.String()))
				continue
			}
		}
	}

	return nil
}

func (h *ScaleHandler) CheckMetricsToScaleDown(ctx context.Context, es *v1alpha1.ElastiService) (bool, error) {
	for _, trigger := range es.Spec.Triggers {
		var scaler scalers.Scaler
		var err error

		switch trigger.Type {
		case "prometheus":
			scaler, err = scalers.NewPrometheusScaler(trigger.Metadata)
		default:
			return false, fmt.Errorf("unsupported trigger type: %s", trigger.Type)
		}

		if err != nil {
			return false, fmt.Errorf("failed to create scaler: %w", err)
		}

		scaleDown, err := scaler.ShouldScaleToZero(ctx)
		scaler.Close(ctx)
		if err != nil {
			return false, fmt.Errorf("failed to check to scale to zero: %w", err)
		}
		if !scaleDown {
			return false, nil
		}
	}
	return true, nil
}

// ScaleTargetWhenAtZero scales the TargetRef to the provided replicas when it's at 0
func (h *ScaleHandler) ScaleTargetWhenAtZero(namespacedName types.NamespacedName, targetKind string, replicas int32) error {
	mutex := h.getMutexForScale(namespacedName.String())
	mutex.Lock()
	defer mutex.Unlock()

	h.logger.Info("Scaling up from zero", zap.String("kind", targetKind), zap.String("namespacedName", namespacedName.String()), zap.Int32("replicas", replicas))
	switch strings.ToLower(targetKind) {
	case values.KindDeployments:
		err := h.ScaleDeployment(namespacedName.Namespace, namespacedName.Name, replicas)
		if err != nil {
			return fmt.Errorf("ScaleTargetWhenAtZero - Deployment: %w", err)
		}
	case values.KindRollout:
		err := h.ScaleArgoRollout(namespacedName.Namespace, namespacedName.Name, replicas)
		if err != nil {
			return fmt.Errorf("ScaleTargetWhenAtZero - Rollout: %w", err)
		}
	default:
		return fmt.Errorf("unsupported target kind: %s", targetKind)
	}
	return nil
}

// ScaleTargetToZero scales the target to zero
// TODO: Emit k8s events
func (h *ScaleHandler) ScaleTargetToZero(namespacedName types.NamespacedName, targetKind string) error {
	mutex := h.getMutexForScale(namespacedName.String())
	mutex.Lock()
	defer mutex.Unlock()

	h.logger.Info("Scaling down to zero", zap.String("kind", targetKind), zap.String("namespacedName", namespacedName.String()))
	switch strings.ToLower(targetKind) {
	case values.KindDeployments:
		err := h.ScaleDeployment(namespacedName.Namespace, namespacedName.Name, 0)
		if err != nil {
			return fmt.Errorf("ScaleTargetToZero - Deployment: %w", err)
		}
	case values.KindRollout:
		err := h.ScaleArgoRollout(namespacedName.Namespace, namespacedName.Name, 0)
		if err != nil {
			return fmt.Errorf("ScaleTargetToZero - Rollout: %w", err)
		}
	default:
		return fmt.Errorf("unsupported target kind: %s", targetKind)
	}
	return nil
}

// ScaleDeployment scales the deployment to the provided replicas
// TODO: use a generic logic to perform scaling similar to HPA/KEDA
func (h *ScaleHandler) ScaleDeployment(ns, targetName string, replicas int32) error {
	deploymentClient := h.kClient.AppsV1().Deployments(ns)
	deploy, err := deploymentClient.Get(context.TODO(), targetName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("ScaleDeployment - GET: %w", err)
	}

	h.logger.Debug("Deployment found", zap.String("deployment", targetName), zap.Int32("current replicas", *deploy.Spec.Replicas), zap.Int32("desired replicas", replicas))
	if deploy.Spec.Replicas != nil && *deploy.Spec.Replicas != replicas {
		*deploy.Spec.Replicas = replicas
		_, err = deploymentClient.Update(context.TODO(), deploy, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("ScaleDeployment - Update: %w", err)
		}
		h.logger.Info("Deployment scaled", zap.String("deployment", targetName), zap.Int32("replicas", replicas))
		return nil
	}
	h.logger.Info("Deployment already scaled", zap.String("deployment", targetName), zap.Int32("current replicas", *deploy.Spec.Replicas))
	return nil
}

// ScaleArgoRollout scales the rollout to the provided replicas
func (h *ScaleHandler) ScaleArgoRollout(ns, targetName string, replicas int32) error {
	rollout, err := h.kDynamicClient.Resource(values.RolloutGVR).Namespace(ns).Get(context.TODO(), targetName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("ScaleArgoRollout - GET: %w", err)
	}

	currentReplicas := rollout.Object["spec"].(map[string]interface{})["replicas"].(int64)
	h.logger.Info("Rollout found", zap.String("rollout", targetName), zap.Int64("current replicas", currentReplicas), zap.Int32("desired replicas", replicas))

	if currentReplicas != int64(replicas) {
		rollout.Object["spec"].(map[string]interface{})["replicas"] = replicas
		_, err = h.kDynamicClient.Resource(values.RolloutGVR).Namespace(ns).Update(context.TODO(), rollout, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("ScaleArgoRollout - Update: %w", err)
		}
		h.logger.Info("Rollout scaled", zap.String("rollout", targetName), zap.Int32("replicas", replicas))
		return nil
	}
	h.logger.Info("Rollout already scaled", zap.String("rollout", targetName), zap.Int64("current replicas", currentReplicas))

	return nil
}

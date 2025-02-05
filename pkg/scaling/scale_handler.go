package scaling

import (
	"context"
	"fmt"
	"os"
	"strconv"
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

const (
	kedaPausedAnnotation = "autoscaling.keda.sh/paused"
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
	pollingInterval := 30 * time.Second
	if envInterval := os.Getenv("POLLING_VARIABLE"); envInterval != "" {
		duration, err := time.ParseDuration(envInterval)
		if err != nil {
			h.logger.Warn("Invalid POLLING_VARIABLE value, using default 30s", zap.Error(err))
		} else {
			pollingInterval = duration
		}
	}
	ticker := time.NewTicker(pollingInterval)

	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				if err := h.checkAndScale(ctx); err != nil {
					h.logger.Error("failed to run the scale down check", zap.Error(err))
				}
			}
		}
	}()
}

func (h *ScaleHandler) checkAndScale(ctx context.Context) error {
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

		if es.Status.Mode == values.ServeMode {
			err := h.checkMetricsAndScale(ctx, es, true)
			if err != nil {
				h.logger.Error("failed to check and scale from zero", zap.Error(err))
				continue
			}
		} else {
			err := h.checkMetricsAndScale(ctx, es, false)
			if err != nil {
				h.logger.Error("failed to check and scale to zero", zap.Error(err))
				continue
			}
		}
	}

	return nil
}

func (h *ScaleHandler) checkMetricsAndScale(ctx context.Context, es *v1alpha1.ElastiService, scaleDown bool) error {
	namespacedName := types.NamespacedName{
		Name:      es.Spec.ScaleTargetRef.Name,
		Namespace: es.Namespace,
	}

	shouldScale, err := h.checkMetrics(ctx, es, scaleDown)
	if err != nil {
		return fmt.Errorf("failed to check metrics to scale down: %w", err)
	}
	if !shouldScale {
		return nil
	}

	// If scaling down, pause the keda scaledobject
	// If scaling up, unpause the keda scaledobject
	if es.Spec.Autoscaler != nil && strings.ToLower(es.Spec.Autoscaler.Type) == "keda" {
		err := h.UpdateKedaScaledObject(ctx, es.Spec.Autoscaler.Name, es.Namespace, scaleDown)
		if err != nil {
			return fmt.Errorf("failed to update Keda ScaledObject for service %s: %w", namespacedName.String(), err)
		}
	}

	if scaleDown {
		if err := h.ScaleTargetToZero(namespacedName, es.Spec.ScaleTargetRef.Kind); err != nil {
			return fmt.Errorf("failed to scale target to zero: %w", err)
		}
	} else {
		if err := h.ScaleTargetFromZero(namespacedName, es.Spec.ScaleTargetRef.Kind, es.Spec.MinTargetReplicas); err != nil {
			return fmt.Errorf("failed to scale target from zero: %w", err)
		}
	}
	return nil
}

func (h *ScaleHandler) checkMetrics(ctx context.Context, es *v1alpha1.ElastiService, scaleDown bool) (bool, error) {
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

		var shouldScale bool
		if scaleDown {
			shouldScale, err = scaler.ShouldScaleToZero(ctx)
		} else {
			shouldScale, err = scaler.ShouldScaleFromZero(ctx)
		}
		if err != nil {
			return false, fmt.Errorf("failed to check scaler: %w", err)
		}

		scaler.Close(ctx)
		if !shouldScale {
			return false, nil
		}
	}
	return true, nil
}

// ScaleTargetFromZero scales the TargetRef to the provided replicas when it's at 0
func (h *ScaleHandler) ScaleTargetFromZero(namespacedName types.NamespacedName, targetKind string, replicas int32) error {
	mutex := h.getMutexForScale(namespacedName.String())
	mutex.Lock()
	defer mutex.Unlock()

	h.logger.Info("Scaling up from zero", zap.String("kind", targetKind), zap.String("namespacedName", namespacedName.String()), zap.Int32("replicas", replicas))
	switch strings.ToLower(targetKind) {
	case values.KindDeployments:
		err := h.ScaleDeployment(namespacedName.Namespace, namespacedName.Name, replicas)
		if err != nil {
			return fmt.Errorf("ScaleTargetFromZero - Deployment: %w", err)
		}
	case values.KindRollout:
		err := h.ScaleArgoRollout(namespacedName.Namespace, namespacedName.Name, replicas)
		if err != nil {
			return fmt.Errorf("ScaleTargetFromZero - Rollout: %w", err)
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

func (h *ScaleHandler) UpdateKedaScaledObject(ctx context.Context, scaledObjectName, namespace string, paused bool) error {
	scaledObject, err := h.kDynamicClient.Resource(values.ScaledObjectGVR).Namespace(namespace).Get(ctx, scaledObjectName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ScaledObject: %w", err)
	}

	annotations := scaledObject.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[kedaPausedAnnotation] = strconv.FormatBool(paused)
	scaledObject.SetAnnotations(annotations)

	_, err = h.kDynamicClient.Resource(values.ScaledObjectGVR).Namespace(namespace).Update(ctx, scaledObject, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ScaledObject: %w", err)
	}

	return nil
}

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
	v1 "k8s.io/api/core/v1"
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
			err := h.handleScaleToZero(ctx, es)
			if err != nil {
				h.logger.Error("failed to check and scale from zero", zap.String("service", es.Spec.Service), zap.String("namespace", es.Namespace), zap.Error(err))
				continue
			}
		} else {
			err := h.handleScaleFromZero(ctx, es)
			if err != nil {
				h.logger.Error("failed to check and scale to zero", zap.String("service", es.Spec.Service), zap.String("namespace", es.Namespace), zap.Error(err))
				continue
			}
		}
	}

	return nil
}

func (h *ScaleHandler) handleScaleToZero(ctx context.Context, es *v1alpha1.ElastiService) error {
	namespacedName := types.NamespacedName{
		Name:      es.Spec.Service,
		Namespace: es.Namespace,
	}
	shouldScale := true
	if len(es.Spec.Triggers) == 0 {
		h.logger.Info("No triggers found, skipping scale to zero", zap.String("namespacedName", namespacedName.String()))
		return nil
	}
	for _, trigger := range es.Spec.Triggers {
		scaler, err := h.createScalerForTrigger(&trigger)
		if err != nil {
			return fmt.Errorf("failed to create scaler for %s: %w", namespacedName.String(), err)
		}

		scalerResult, err := scaler.ShouldScaleToZero(ctx)
		if err != nil {
			return fmt.Errorf("failed to check scaler for %s: %w", namespacedName.String(), err)
		}
		if !scalerResult {
			shouldScale = false
			break
		}

		err = scaler.Close(ctx)
		if err != nil {
			h.logger.Error("failed to close scaler", zap.String("namespacedName", namespacedName.String()), zap.Error(err))
		}
	}
	if !shouldScale {
		return nil
	}

	// Pause the KEDA ScaledObject
	if es.Spec.Autoscaler != nil && strings.ToLower(es.Spec.Autoscaler.Type) == "keda" {
		err := h.UpdateKedaScaledObjectPausedState(ctx, es.Spec.Autoscaler.Name, es.Namespace, true)
		if err != nil {
			return fmt.Errorf("failed to update Keda ScaledObject for service %s: %w", namespacedName.String(), err)
		}
	}

	// Check cooldown period
	if es.Status.LastScaledUpTime != nil {
		cooldownPeriod := time.Second * time.Duration(es.Spec.CooldownPeriod)
		if cooldownPeriod == 0 {
			cooldownPeriod = values.DefaultCooldownPeriod
		}

		if time.Since(es.Status.LastScaledUpTime.Time) < cooldownPeriod {
			h.logger.Info("Skipping scale down as minimum cooldownPeriod not met",
				zap.String("service", namespacedName.String()),
				zap.Duration("cooldown", cooldownPeriod),
				zap.Time("last scaled up time", es.Status.LastScaledUpTime.Time))
			return nil
		}
	}

	if err := h.ScaleTargetToZero(namespacedName, es.Spec.ScaleTargetRef.Kind, es.Name); err != nil {
		return fmt.Errorf("failed to scale target to zero: %w", err)
	}
	return nil
}

func (h *ScaleHandler) handleScaleFromZero(ctx context.Context, es *v1alpha1.ElastiService) error {
	namespacedName := types.NamespacedName{
		Name:      es.Spec.Service,
		Namespace: es.Namespace,
	}
	shouldScale := false
	if len(es.Spec.Triggers) == 0 {
		h.logger.Info("No triggers found, skipping scale from zero", zap.String("namespacedName", namespacedName.String()))
		return nil
	}
	for _, trigger := range es.Spec.Triggers {
		scaler, err := h.createScalerForTrigger(&trigger)
		if err != nil {
			h.logger.Error("failed to create scaler for trigger", zap.String("namespacedName", namespacedName.String()), zap.Error(err))
			shouldScale = true
			break
		}

		scalerResult, err := scaler.ShouldScaleFromZero(ctx)
		if err != nil {
			h.logger.Error("failed to check scaler", zap.String("namespacedName", namespacedName.String()), zap.Error(err))
			shouldScale = true
			break
		}
		if scalerResult {
			shouldScale = true
			break
		}

		err = scaler.Close(ctx)
		if err != nil {
			h.logger.Error("failed to close scaler", zap.String("namespacedName", namespacedName.String()), zap.Error(err))
		}
	}
	if !shouldScale {
		return nil
	}

	// Unpause the KEDA ScaledObject if it's paused
	if es.Spec.Autoscaler != nil && strings.ToLower(es.Spec.Autoscaler.Type) == "keda" {
		err := h.UpdateKedaScaledObjectPausedState(ctx, es.Spec.Autoscaler.Name, es.Namespace, false)
		if err != nil {
			return fmt.Errorf("failed to update Keda ScaledObject for service %s: %w", namespacedName.String(), err)
		}
	}

	if err := h.ScaleTargetFromZero(namespacedName, es.Spec.ScaleTargetRef.Kind, es.Spec.MinTargetReplicas, es.Name); err != nil {
		return fmt.Errorf("failed to scale target from zero: %w", err)
	}

	return nil
}

func (h *ScaleHandler) createScalerForTrigger(trigger *v1alpha1.ScaleTrigger) (scalers.Scaler, error) {
	var scaler scalers.Scaler
	var err error

	switch trigger.Type {
	case "prometheus":
		scaler, err = scalers.NewPrometheusScaler(trigger.Metadata)
	default:
		return nil, fmt.Errorf("unsupported trigger type: %s", trigger.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create scaler: %w", err)
	}
	return scaler, nil
}

// ScaleTargetFromZero scales the TargetRef to the provided replicas when it's at 0
func (h *ScaleHandler) ScaleTargetFromZero(namespacedName types.NamespacedName, targetKind string, replicas int32, elastiServiceName string) error {
	mutex := h.getMutexForScale(namespacedName.String())
	mutex.Lock()
	defer mutex.Unlock()

	h.logger.Info("Scaling up from zero", zap.String("kind", targetKind), zap.String("namespacedName", namespacedName.String()), zap.Int32("replicas", replicas))

	var err error
	switch strings.ToLower(targetKind) {
	case values.KindDeployments:
		err = h.ScaleDeployment(namespacedName.Namespace, namespacedName.Name, replicas)
	case values.KindRollout:
		err = h.ScaleArgoRollout(namespacedName.Namespace, namespacedName.Name, replicas)
	default:
		return fmt.Errorf("unsupported target kind: %s", targetKind)
	}

	if err != nil {
		eventErr := h.createEvent(namespacedName.Namespace, elastiServiceName, "Warning", "ScaleFromZeroFailed", fmt.Sprintf("Failed to scale %s from zero to %d replicas: %v", targetKind, replicas, err))
		if eventErr != nil {
			h.logger.Error("Failed to create failure event", zap.Error(eventErr))
		}
		return fmt.Errorf("ScaleTargetFromZero - %s: %w", targetKind, err)
	}

	eventErr := h.createEvent(namespacedName.Namespace, elastiServiceName, "Normal", "ScaledUpFromZero", fmt.Sprintf("Successfully scaled %s from zero to %d replicas", targetKind, replicas))
	if eventErr != nil {
		h.logger.Error("Failed to create success event", zap.Error(eventErr))
	}

	if err := h.UpdateLastScaledUpTime(context.Background(), elastiServiceName, namespacedName.Namespace); err != nil {
		h.logger.Error("Failed to update LastScaledUpTime", zap.Error(err), zap.String("namespacedName", namespacedName.String()))
	}

	return nil
}

// ScaleTargetToZero scales the target to zero
func (h *ScaleHandler) ScaleTargetToZero(namespacedName types.NamespacedName, targetKind string, elastiServiceName string) error {
	mutex := h.getMutexForScale(namespacedName.String())
	mutex.Lock()
	defer mutex.Unlock()

	h.logger.Info("Scaling down to zero", zap.String("kind", targetKind), zap.String("namespacedName", namespacedName.String()))

	var err error
	switch strings.ToLower(targetKind) {
	case values.KindDeployments:
		err = h.ScaleDeployment(namespacedName.Namespace, namespacedName.Name, 0)
	case values.KindRollout:
		err = h.ScaleArgoRollout(namespacedName.Namespace, namespacedName.Name, 0)
	default:
		return fmt.Errorf("unsupported target kind: %s", targetKind)
	}

	if err != nil {
		eventErr := h.createEvent(namespacedName.Namespace, elastiServiceName, "Warning", "ScaleToZeroFailed", fmt.Sprintf("Failed to scale %s to zero: %v", targetKind, err))
		if eventErr != nil {
			h.logger.Error("Failed to create failure event", zap.Error(eventErr))
		}
		return fmt.Errorf("ScaleTargetToZero - %s: %w", targetKind, err)
	}

	eventErr := h.createEvent(namespacedName.Namespace, elastiServiceName, "Normal", "ScaledDownToZero", fmt.Sprintf("Successfully scaled %s to zero", targetKind))
	if eventErr != nil {
		h.logger.Error("Failed to create success event", zap.Error(eventErr))
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

func (h *ScaleHandler) UpdateKedaScaledObjectPausedState(ctx context.Context, scaledObjectName, namespace string, paused bool) error {
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

func (h *ScaleHandler) UpdateLastScaledUpTime(ctx context.Context, crdName, namespace string) error {
	now := metav1.Now()
	patchBytes := []byte(fmt.Sprintf(`{"status": {"lastScaledUpTime": "%s"}}`, now.Format(time.RFC3339Nano)))

	_, err := h.kDynamicClient.Resource(values.ElastiServiceGVR).
		Namespace(namespace).
		Patch(ctx, crdName, types.MergePatchType, patchBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		return fmt.Errorf("failed to patch ElastiService status: %w", err)
	}
	return nil
}

// createEvent creates a new event on scaling up or down
func (h *ScaleHandler) createEvent(namespace, name, eventType, reason, message string) error {
	h.logger.Info("createEvent", zap.String("eventType", eventType), zap.String("reason", reason), zap.String("message", message))
	event := &v1.Event{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: name + "-",
			Namespace:    namespace,
		},
		InvolvedObject: v1.ObjectReference{
			APIVersion: "elasti.truefoundry.com/v1alpha1",
			Kind:       "ElastiService",
			Name:       name,
			Namespace:  namespace,
		},
		Type:    eventType, // Normal or Warning
		Reason:  reason,
		Message: message,
		Action:  "Scale",
		Source: v1.EventSource{
			Component: "elasti-operator",
		},
	}

	_, err := h.kClient.CoreV1().Events(namespace).Create(context.TODO(), event, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}
	return nil
}

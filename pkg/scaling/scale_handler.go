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
	kedaPausedAnnotation         = "autoscaling.keda.sh/paused"
	kedaPausedReplicasAnnotation = "autoscaling.keda.sh/paused-replicas"
)

type ScaleDirection string

const (
	ScaleUp   ScaleDirection = "scale-up"
	ScaleDown ScaleDirection = "scale-down"
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
	if err := h.checkAndScale(ctx); err != nil {
		h.logger.Error("failed to run the scale down check", zap.Error(err))
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
	elastiServiceList, err := h.kDynamicClient.Resource(values.ElastiServiceGVR).List(ctx, metav1.ListOptions{
		FieldSelector: "metadata.name=elasti-trigger-failure-elasti-service",
	})
	if err != nil {
		return fmt.Errorf("failed to list ElastiServices: %w", err)
	}

	for _, item := range elastiServiceList.Items {
		es := &v1alpha1.ElastiService{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, es); err != nil {
			h.logger.Error("failed to convert unstructured to ElastiService", zap.Error(err))
			continue
		}

		scaleDirection, err := h.calculateScaleDirection(ctx, es)
		if err != nil {
			h.logger.Error("failed to calculate scale direction", zap.String("service", es.Spec.Service), zap.String("namespace", es.Namespace), zap.Error(err))
			continue
		}
		if scaleDirection == ScaleDown {
			err := h.handleScaleToZero(ctx, es)
			if err != nil {
				h.logger.Error("failed to scale target to zero", zap.String("service", es.Spec.Service), zap.String("namespace", es.Namespace), zap.Error(err))
				continue
			}
		} else if scaleDirection == ScaleUp {
			err := h.handleScaleFromZero(ctx, es)
			if err != nil {
				h.logger.Error("failed to scale target from zero", zap.String("service", es.Spec.Service), zap.String("namespace", es.Namespace), zap.Error(err))
				continue
			}
		} else {
			h.logger.Error("failed to calculate scale direction", zap.String("service", es.Spec.Service), zap.String("namespace", es.Namespace), zap.Error(err))
			continue
		}
	}

	return nil
}

func (h *ScaleHandler) calculateScaleDirection(ctx context.Context, es *v1alpha1.ElastiService) (ScaleDirection, error) {
	if len(es.Spec.Triggers) == 0 {
		h.logger.Info("No triggers found, skipping scale to zero", zap.String("namespacedName", es.Namespace+"/"+es.Spec.Service))
		return ScaleDown, fmt.Errorf("no triggers found")
	}

	for _, trigger := range es.Spec.Triggers {
		scaler, err := h.createScalerForTrigger(&trigger)
		if err != nil {
			h.logger.Warn("failed to create scaler", zap.String("namespacedName", es.Namespace+"/"+es.Spec.Service), zap.Error(err))
			return "", fmt.Errorf("failed to create scaler: %w", err)
		}

		scaleToZero, err := scaler.ShouldScaleToZero(ctx)
		if err != nil {
			h.logger.Warn("failed to check scaler", zap.String("namespacedName", es.Namespace+"/"+es.Spec.Service), zap.Error(err))
			return "", fmt.Errorf("failed to check scaler: %w", err)
		}
		if err = scaler.Close(ctx); err != nil {
			h.logger.Error("failed to close scaler", zap.String("namespacedName", es.Namespace+"/"+es.Spec.Service), zap.Error(err))
		}

		if !scaleToZero {
			return ScaleUp, nil
		}
	}

	return ScaleDown, nil
}

func (h *ScaleHandler) handleScaleToZero(ctx context.Context, es *v1alpha1.ElastiService) error {
	namespacedName := types.NamespacedName{
		Name:      es.Spec.Service,
		Namespace: es.Namespace,
	}

	if es.Status.LastScaledUpTime != nil {
		cooldownPeriod := time.Second * time.Duration(es.Spec.CooldownPeriod)
		if cooldownPeriod == 0 {
			cooldownPeriod = values.DefaultCooldownPeriod
		}

		if time.Since(es.Status.LastScaledUpTime.Time) < cooldownPeriod {
			h.logger.Debug("Skipping scale down as minimum cooldownPeriod not met",
				zap.String("service", namespacedName.String()),
				zap.Duration("cooldown", cooldownPeriod),
				zap.Time("last scaled up time", es.Status.LastScaledUpTime.Time))
			return nil
		}
	}

	// Pause the KEDA ScaledObject
	if es.Spec.Autoscaler != nil && strings.ToLower(es.Spec.Autoscaler.Type) == "keda" {
		err := h.UpdateKedaScaledObjectPausedState(ctx, es.Spec.Autoscaler.Name, es.Namespace, true)
		if err != nil {
			return fmt.Errorf("failed to update Keda ScaledObject for service %s: %w", namespacedName.String(), err)
		}
	}

	if err := h.ScaleTargetToZero(ctx, namespacedName, es.Spec.ScaleTargetRef.Kind, es.Name); err != nil {
		return fmt.Errorf("failed to scale target to zero: %w", err)
	}
	return nil
}

func (h *ScaleHandler) handleScaleFromZero(ctx context.Context, es *v1alpha1.ElastiService) error {
	namespacedName := types.NamespacedName{
		Name:      es.Spec.Service,
		Namespace: es.Namespace,
	}

	// We update the last scaled up time every time we evaluate that the trigger evaluates to scale-up
	if err := h.UpdateLastScaledUpTime(ctx, es.Name, es.Namespace); err != nil {
		h.logger.Error("Failed to update LastScaledUpTime", zap.Error(err), zap.String("namespacedName", namespacedName.String()))
	}

	// Unpause the KEDA ScaledObject if it's paused
	if es.Spec.Autoscaler != nil && strings.ToLower(es.Spec.Autoscaler.Type) == "keda" {
		err := h.UpdateKedaScaledObjectPausedState(ctx, es.Spec.Autoscaler.Name, es.Namespace, false)
		if err != nil {
			return fmt.Errorf("failed to update Keda ScaledObject for service %s: %w", namespacedName.String(), err)
		}
	}

	if err := h.ScaleTargetFromZero(ctx, namespacedName, es.Spec.ScaleTargetRef.Kind, es.Spec.MinTargetReplicas, es.Name); err != nil {
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
func (h *ScaleHandler) ScaleTargetFromZero(ctx context.Context, namespacedName types.NamespacedName, targetKind string, replicas int32, elastiServiceName string) error {
	mutex := h.getMutexForScale(namespacedName.String())
	mutex.Lock()
	defer mutex.Unlock()

	h.logger.Info("Scaling up from zero", zap.String("kind", targetKind), zap.String("namespacedName", namespacedName.String()), zap.Int32("replicas", replicas))

	var err error
	var scaled bool = false
	switch strings.ToLower(targetKind) {
	case values.KindDeployments:
		scaled, err = h.ScaleDeployment(ctx, namespacedName.Namespace, namespacedName.Name, replicas)
	case values.KindRollout:
		scaled, err = h.ScaleArgoRollout(ctx, namespacedName.Namespace, namespacedName.Name, replicas)
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

	if !scaled {
		// Returning nil as the scale target is already scaled
		return nil
	}

	eventErr := h.createEvent(namespacedName.Namespace, elastiServiceName, "Normal", "ScaledUpFromZero", fmt.Sprintf("Successfully scaled %s from zero to %d replicas", targetKind, replicas))
	if eventErr != nil {
		h.logger.Error("Failed to create success event", zap.Error(eventErr))
	}

	return nil
}

// ScaleTargetToZero scales the target to zero
func (h *ScaleHandler) ScaleTargetToZero(ctx context.Context, namespacedName types.NamespacedName, targetKind string, elastiServiceName string) error {
	mutex := h.getMutexForScale(namespacedName.String())
	mutex.Lock()
	defer mutex.Unlock()

	h.logger.Info("Scaling down to zero", zap.String("kind", targetKind), zap.String("namespacedName", namespacedName.String()))

	var err error
	var scaled bool = false
	switch strings.ToLower(targetKind) {
	case values.KindDeployments:
		scaled, err = h.ScaleDeployment(ctx, namespacedName.Namespace, namespacedName.Name, 0)
	case values.KindRollout:
		scaled, err = h.ScaleArgoRollout(ctx, namespacedName.Namespace, namespacedName.Name, 0)
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

	if !scaled {
		// Returning nil as the scale target is already scaled down to zero
		return nil
	}

	eventErr := h.createEvent(namespacedName.Namespace, elastiServiceName, "Normal", "ScaledDownToZero", fmt.Sprintf("Successfully scaled %s to zero", targetKind))
	if eventErr != nil {
		h.logger.Error("Failed to create success event", zap.Error(eventErr))
	}

	return nil
}

// ScaleDeployment scales the deployment to the provided replicas
// TODO: use a generic logic to perform scaling similar to HPA/KEDA
func (h *ScaleHandler) ScaleDeployment(ctx context.Context, ns, targetName string, replicas int32) (bool, error) {
	deploymentClient := h.kClient.AppsV1().Deployments(ns)
	deploy, err := deploymentClient.Get(ctx, targetName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("ScaleDeployment - GET: %w", err)
	}

	h.logger.Debug("Deployment found", zap.String("deployment", targetName), zap.Int32("current replicas", *deploy.Spec.Replicas), zap.Int32("desired replicas", replicas))
	if deploy.Spec.Replicas != nil && *deploy.Spec.Replicas != replicas {
		patchBytes := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas))
		_, err = deploymentClient.Patch(ctx, targetName, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			return false, fmt.Errorf("ScaleDeployment - Patch: %w", err)
		}
		h.logger.Info("Deployment scaled", zap.String("deployment", targetName), zap.Int32("replicas", replicas))
		return true, nil
	}
	h.logger.Info("Deployment already scaled", zap.String("deployment", targetName), zap.Int32("current replicas", *deploy.Spec.Replicas))
	return false, nil
}

// ScaleArgoRollout scales the rollout to the provided replicas
func (h *ScaleHandler) ScaleArgoRollout(ctx context.Context, ns, targetName string, replicas int32) (bool, error) {
	rollout, err := h.kDynamicClient.Resource(values.RolloutGVR).Namespace(ns).Get(ctx, targetName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("ScaleArgoRollout - GET: %w", err)
	}

	currentReplicas := rollout.Object["spec"].(map[string]interface{})["replicas"].(int64)
	h.logger.Info("Rollout found", zap.String("rollout", targetName), zap.Int64("current replicas", currentReplicas), zap.Int32("desired replicas", replicas))

	if currentReplicas != int64(replicas) {
		patchBytes := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas))
		_, err = h.kDynamicClient.Resource(values.RolloutGVR).Namespace(ns).Patch(
			ctx,
			targetName,
			types.MergePatchType,
			patchBytes,
			metav1.PatchOptions{},
		)
		if err != nil {
			return false, fmt.Errorf("ScaleArgoRollout - Patch: %w", err)
		}
		h.logger.Info("Rollout scaled", zap.String("rollout", targetName), zap.Int32("replicas", replicas))
		return true, nil
	}
	h.logger.Info("Rollout already scaled", zap.String("rollout", targetName), zap.Int64("current replicas", currentReplicas))
	return false, nil
}

func (h *ScaleHandler) UpdateKedaScaledObjectPausedState(ctx context.Context, scaledObjectName, namespace string, paused bool) error {
	var patchBytes []byte
	if paused {
		// When pausing, set both annotations: paused=true and paused-replicas="0"
		patchBytes = []byte(fmt.Sprintf(`{"metadata": {"annotations": {"%s": "%s", "%s": "0"}}}`,
			kedaPausedAnnotation,
			strconv.FormatBool(paused),
			kedaPausedReplicasAnnotation))
	} else {
		// When unpausing, set paused=false and remove the paused-replicas annotation
		patchBytes = []byte(fmt.Sprintf(`{"metadata": {"annotations": {"%s": "%s", "%s": null}}}`,
			kedaPausedAnnotation,
			strconv.FormatBool(paused),
			kedaPausedReplicasAnnotation))
	}

	_, err := h.kDynamicClient.Resource(values.ScaledObjectGVR).Namespace(namespace).Patch(
		ctx,
		scaledObjectName,
		types.MergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to patch ScaledObject: %w", err)
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

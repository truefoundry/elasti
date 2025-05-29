package controller

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
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	kedaPausedAnnotation         = "autoscaling.keda.sh/paused"
	kedaPausedReplicasAnnotation = "autoscaling.keda.sh/paused-replicas"
)

// getMutexForScale returns a mutex for scaling based on the input key
func (h *ElastiServiceReconciler) getMutexForScale(key string) *sync.Mutex {
	l, _ := h.ScaleLocks.LoadOrStore(key, &sync.Mutex{})
	return l.(*sync.Mutex)
}

func (h *ElastiServiceReconciler) StartScaleDownWatcher(ctx context.Context) {
	pollingInterval := 30 * time.Second
	if envInterval := os.Getenv("POLLING_VARIABLE"); envInterval != "" {
		duration, err := time.ParseDuration(envInterval)
		if err != nil {
			h.Logger.Warn("Invalid POLLING_VARIABLE value, using default 30s", zap.Error(err))
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
					h.Logger.Error("failed to run the scale down check", zap.Error(err))
				}
			}
		}
	}()
}

func (h *ElastiServiceReconciler) checkAndScale(ctx context.Context) error {
	elastiServiceList := &v1alpha1.ElastiServiceList{}
	if err := h.List(ctx, elastiServiceList, client.InNamespace("shub-ws")); err != nil {
		return fmt.Errorf("failed to list ElastiServices: %w", err)
	}

	for _, es := range elastiServiceList.Items {
		if es.Status.Mode == values.ServeMode {
			err := h.handleScaleToZero(ctx, &es)
			if err != nil {
				h.Logger.Error("failed to check and scale from zero", zap.String("service", es.Spec.Service), zap.String("namespace", es.Namespace), zap.Error(err))
				continue
			}
		} else {
			err := h.handleScaleFromZero(ctx, &es)
			if err != nil {
				h.Logger.Error("failed to check and scale to zero", zap.String("service", es.Spec.Service), zap.String("namespace", es.Namespace), zap.Error(err))
				continue
			}
		}
	}

	return nil
}

func (h *ElastiServiceReconciler) handleScaleToZero(ctx context.Context, es *v1alpha1.ElastiService) error {
	namespacedName := types.NamespacedName{
		Name:      es.Spec.Service,
		Namespace: es.Namespace,
	}
	shouldScale := true
	if len(es.Spec.Triggers) == 0 {
		h.Logger.Info("No triggers found, skipping scale to zero", zap.String("namespacedName", namespacedName.String()))
		return nil
	}
	for _, trigger := range es.Spec.Triggers {
		scaler, err := h.createScalerForTrigger(&trigger)
		if err != nil {
			// Return nil as this error cannot be fixed or acted upon by elasti
			h.Logger.Warn("failed to create scaler", zap.String("namespacedName", namespacedName.String()), zap.Error(err))
			return nil
		}

		scalerResult, err := scaler.ShouldScaleToZero(ctx)
		if err != nil {
			// Return nil as this error cannot be fixed or acted upon by elasti
			h.Logger.Warn("failed to check scaler", zap.String("namespacedName", namespacedName.String()), zap.Error(err))
			return nil
		}
		if !scalerResult {
			shouldScale = false
			break
		}

		err = scaler.Close(ctx)
		if err != nil {
			h.Logger.Error("failed to close scaler", zap.String("namespacedName", namespacedName.String()), zap.Error(err))
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
			h.Logger.Debug("Skipping scale down as minimum cooldownPeriod not met",
				zap.String("service", namespacedName.String()),
				zap.Duration("cooldown", cooldownPeriod),
				zap.Time("last scaled up time", es.Status.LastScaledUpTime.Time))
			return nil
		}
	}

	if err := h.ScaleTargetToZero(ctx, namespacedName, es.Spec.ScaleTargetRef.Kind, es.Name); err != nil {
		return fmt.Errorf("failed to scale target to zero: %w", err)
	}
	return nil
}

func (h *ElastiServiceReconciler) handleScaleFromZero(ctx context.Context, es *v1alpha1.ElastiService) error {
	namespacedName := types.NamespacedName{
		Name:      es.Spec.Service,
		Namespace: es.Namespace,
	}
	shouldScale := false
	if len(es.Spec.Triggers) == 0 {
		h.Logger.Info("No triggers found, skipping scale from zero", zap.String("namespacedName", namespacedName.String()))
		return nil
	}
	for _, trigger := range es.Spec.Triggers {
		scaler, err := h.createScalerForTrigger(&trigger)
		if err != nil {
			h.Logger.Warn("failed to create scaler for trigger", zap.String("namespacedName", namespacedName.String()), zap.Error(err))
			shouldScale = true
			break
		}

		scalerResult, err := scaler.ShouldScaleFromZero(ctx)
		if err != nil {
			h.Logger.Warn("failed to check scaler", zap.String("namespacedName", namespacedName.String()), zap.Error(err))
			break
		}
		if scalerResult {
			shouldScale = true
			break
		}

		err = scaler.Close(ctx)
		if err != nil {
			h.Logger.Error("failed to close scaler", zap.String("namespacedName", namespacedName.String()), zap.Error(err))
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

	if err := h.ScaleTargetFromZero(ctx, namespacedName, es.Spec.ScaleTargetRef.Kind, es.Spec.MinTargetReplicas, es.Name); err != nil {
		return fmt.Errorf("failed to scale target from zero: %w", err)
	}

	return nil
}

func (h *ElastiServiceReconciler) createScalerForTrigger(trigger *v1alpha1.ScaleTrigger) (scalers.Scaler, error) {
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
func (h *ElastiServiceReconciler) ScaleTargetFromZero(ctx context.Context, namespacedName types.NamespacedName, targetKind string, replicas int32, elastiServiceName string) error {
	mutex := h.getMutexForScale(namespacedName.String())
	mutex.Lock()
	defer mutex.Unlock()

	h.Logger.Info("Scaling up from zero", zap.String("kind", targetKind), zap.String("namespacedName", namespacedName.String()), zap.Int32("replicas", replicas))

	var err error
	switch strings.ToLower(targetKind) {
	case values.KindDeployments:
		err = h.ScaleDeployment(ctx, namespacedName.Namespace, namespacedName.Name, replicas)
	case values.KindRollout:
		err = h.ScaleArgoRollout(ctx, namespacedName.Namespace, namespacedName.Name, replicas)
	default:
		return fmt.Errorf("unsupported target kind: %s", targetKind)
	}

	if err != nil {
		eventErr := h.createEvent(ctx, namespacedName.Namespace, elastiServiceName, "Warning", "ScaleFromZeroFailed", fmt.Sprintf("Failed to scale %s from zero to %d replicas: %v", targetKind, replicas, err))
		if eventErr != nil {
			h.Logger.Error("Failed to create failure event", zap.Error(eventErr))
		}
		return fmt.Errorf("ScaleTargetFromZero - %s: %w", targetKind, err)
	}

	eventErr := h.createEvent(ctx, namespacedName.Namespace, elastiServiceName, "Normal", "ScaledUpFromZero", fmt.Sprintf("Successfully scaled %s from zero to %d replicas", targetKind, replicas))
	if eventErr != nil {
		h.Logger.Error("Failed to create success event", zap.Error(eventErr))
	}

	if err := h.updateLastScaledUpTime(ctx, elastiServiceName, namespacedName.Namespace); err != nil {
		h.Logger.Error("Failed to update LastScaledUpTime", zap.Error(err), zap.String("namespacedName", namespacedName.String()))
	}

	return nil
}

// ScaleTargetToZero scales the target to zero
func (h *ElastiServiceReconciler) ScaleTargetToZero(ctx context.Context, namespacedName types.NamespacedName, targetKind string, elastiServiceName string) error {
	mutex := h.getMutexForScale(namespacedName.String())
	mutex.Lock()
	defer mutex.Unlock()

	h.Logger.Info("Scaling down to zero", zap.String("kind", targetKind), zap.String("namespacedName", namespacedName.String()))

	var err error
	switch strings.ToLower(targetKind) {
	case values.KindDeployments:
		err = h.ScaleDeployment(ctx, namespacedName.Namespace, namespacedName.Name, 0)
	case values.KindRollout:
		err = h.ScaleArgoRollout(ctx, namespacedName.Namespace, namespacedName.Name, 0)
	default:
		return fmt.Errorf("unsupported target kind: %s", targetKind)
	}

	if err != nil {
		eventErr := h.createEvent(ctx, namespacedName.Namespace, elastiServiceName, "Warning", "ScaleToZeroFailed", fmt.Sprintf("Failed to scale %s to zero: %v", targetKind, err))
		if eventErr != nil {
			h.Logger.Error("Failed to create failure event", zap.Error(eventErr))
		}
		return fmt.Errorf("ScaleTargetToZero - %s: %w", targetKind, err)
	}

	eventErr := h.createEvent(ctx, namespacedName.Namespace, elastiServiceName, "Normal", "ScaledDownToZero", fmt.Sprintf("Successfully scaled %s to zero", targetKind))
	if eventErr != nil {
		h.Logger.Error("Failed to create success event", zap.Error(eventErr))
	}

	return nil
}

// ScaleDeployment scales the deployment to the provided replicas
// TODO: use a generic logic to perform scaling similar to HPA/KEDA
func (h *ElastiServiceReconciler) ScaleDeployment(ctx context.Context, ns, targetName string, replicas int32) error {
	deploy := &appsv1.Deployment{}
	if err := h.Get(ctx, types.NamespacedName{Namespace: ns, Name: targetName}, deploy); err != nil {
		return fmt.Errorf("ScaleDeployment - GET: %w", err)
	}

	h.Logger.Debug("Deployment found", zap.String("deployment", targetName), zap.Int32("current replicas", *deploy.Spec.Replicas), zap.Int32("desired replicas", replicas))
	if deploy.Spec.Replicas != nil && *deploy.Spec.Replicas != replicas {
		patchBytes := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas))
		err := h.Patch(ctx, deploy, client.RawPatch(types.MergePatchType, patchBytes))
		if err != nil {
			return fmt.Errorf("ScaleDeployment - Patch: %w", err)
		}
		h.Logger.Info("Deployment scaled", zap.String("deployment", targetName), zap.Int32("replicas", replicas))
		return nil
	}
	h.Logger.Info("Deployment already scaled", zap.String("deployment", targetName), zap.Int32("current replicas", *deploy.Spec.Replicas))
	return nil
}

// ScaleArgoRollout scales the rollout to the provided replicas
func (h *ElastiServiceReconciler) ScaleArgoRollout(ctx context.Context, ns, targetName string, replicas int32) error {
	rollout := &unstructured.Unstructured{}
	rollout.SetGroupVersionKind(values.RolloutGVK)

	if err := h.Get(ctx, types.NamespacedName{Namespace: ns, Name: targetName}, rollout); err != nil {
		return fmt.Errorf("ScaleArgoRollout - GET: %w", err)
	}

	currentReplicas, _, err := unstructured.NestedInt64(rollout.Object, "spec", "replicas")
	if err != nil {
		return fmt.Errorf("ScaleArgoRollout - failed to get replicas: %w", err)
	}

	h.Logger.Info("Rollout found", zap.String("rollout", targetName), zap.Int64("current replicas", currentReplicas), zap.Int32("desired replicas", replicas))

	if currentReplicas != int64(replicas) {
		patchBytes := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas))
		if err := h.Patch(ctx, rollout, client.RawPatch(types.MergePatchType, patchBytes)); err != nil {
			return fmt.Errorf("ScaleArgoRollout - Patch: %w", err)
		}
		h.Logger.Info("Rollout scaled", zap.String("rollout", targetName), zap.Int32("replicas", replicas))
		return nil
	}
	h.Logger.Info("Rollout already scaled", zap.String("rollout", targetName), zap.Int64("current replicas", currentReplicas))
	return nil
}

func (h *ElastiServiceReconciler) UpdateKedaScaledObjectPausedState(ctx context.Context, scaledObjectName, namespace string, paused bool) error {
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

	if err := h.Patch(ctx, &v1alpha1.ElastiService{}, client.RawPatch(types.MergePatchType, patchBytes)); err != nil {
		return fmt.Errorf("failed to patch ElastiService: %w", err)
	}
	return nil
}

func (h *ElastiServiceReconciler) updateLastScaledUpTime(ctx context.Context, crdName, namespace string) error {
	now := metav1.Now()
	patchBytes := []byte(fmt.Sprintf(`{"status": {"lastScaledUpTime": "%s"}}`, now.Format(time.RFC3339Nano)))

	elastiService := &v1alpha1.ElastiService{}
	if err := h.Get(ctx, types.NamespacedName{Namespace: namespace, Name: crdName}, elastiService); err != nil {
		return fmt.Errorf("failed to get ElastiService: %w", err)
	}

	if err := h.Patch(ctx, elastiService, client.RawPatch(types.MergePatchType, patchBytes)); err != nil {
		return fmt.Errorf("failed to patch ElastiService status: %w", err)
	}
	return nil
}

// createEvent creates a new event on scaling up or down
func (h *ElastiServiceReconciler) createEvent(ctx context.Context, namespace, name, eventType, reason, message string) error {
	h.Logger.Info("createEvent", zap.String("eventType", eventType), zap.String("reason", reason), zap.String("message", message))
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

	if err := h.Create(ctx, event); err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}
	return nil
}

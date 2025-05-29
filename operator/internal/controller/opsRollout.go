package controller

import (
	"context"
	"fmt"

	"truefoundry/elasti/operator/api/v1alpha1"

	argo "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/truefoundry/elasti/pkg/k8shelper"
	"github.com/truefoundry/elasti/pkg/values"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
)

func (r *ElastiServiceReconciler) handleTargetRolloutChanges(ctx context.Context, obj interface{}, elastiServiceNamespacedName types.NamespacedName, es *v1alpha1.ElastiService) error {
	newRollout := &argo.Rollout{}
	err := k8shelper.UnstructuredToResource(obj, newRollout)
	if err != nil {
		return fmt.Errorf("failed to convert unstructured to rollout: %w", err)
	}
	replicas := newRollout.Status.ReadyReplicas
	condition := newRollout.Status.Phase
	if replicas == 0 {
		r.Logger.Debug("ScaleTargetRef Rollout has 0 replicas", zap.String("rollout_name", es.Spec.ScaleTargetRef.Name), zap.String("es", elastiServiceNamespacedName.String()))
		if err := r.switchMode(ctx, elastiServiceNamespacedName, values.ProxyMode); err != nil {
			return fmt.Errorf("failed to switch mode: %w", err)
		}
	} else if replicas > 0 && condition == values.ArgoPhaseHealthy {
		r.Logger.Debug("ScaleTargetRef Deployment has ready replicas", zap.String("rollout_name", es.Spec.ScaleTargetRef.Name), zap.String("es", elastiServiceNamespacedName.String()))
		if err := r.switchMode(ctx, elastiServiceNamespacedName, values.ServeMode); err != nil {
			return fmt.Errorf("failed to switch mode: %w", err)
		}
	}
	return nil
}

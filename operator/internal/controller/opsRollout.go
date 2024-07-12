package controller

import (
	"context"

	argo "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/truefoundry/elasti/pkg/k8sHelper"
	"github.com/truefoundry/elasti/pkg/values"
	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
	"truefoundry.io/elasti/api/v1alpha1"
)

func (r *ElastiServiceReconciler) handleTargetRolloutChanges(ctx context.Context, obj interface{}, es *v1alpha1.ElastiService, req ctrl.Request) {
	newRollout := &argo.Rollout{}
	err := k8sHelper.UnstructuredToResource(obj, newRollout)
	if err != nil {
		r.Logger.Error("Failed to convert unstructured to rollout", zap.Error(err))
		return
	}
	replicas := newRollout.Status.ReadyReplicas
	condition := newRollout.Status.Phase
	if replicas == 0 {
		r.Logger.Debug("ScaleTargetRef Rollout has 0 replicas", zap.String("rollout_name", es.Spec.ScaleTargetRef.Name), zap.String("es", req.String()))
		r.switchMode(ctx, req, values.ProxyMode)
	} else if replicas > 0 && condition == values.ArgoPhaseHealthy {
		r.Logger.Debug("ScaleTargetRef Deployment has ready replicas", zap.String("rollout_name", es.Spec.ScaleTargetRef.Name), zap.String("es", req.String()))
		r.switchMode(ctx, req, values.ServeMode)
	}
}

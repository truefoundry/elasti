package controller

import (
	"context"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

func (r *ElastiServiceReconciler) getModeFromDeployment(ctx context.Context, deploymentNamespacedName types.NamespacedName) (string, error) {
	depl := &appsv1.Deployment{}
	if err := r.Get(ctx, deploymentNamespacedName, depl); err != nil {
		if errors.IsNotFound(err) {
			r.Logger.Info("Deployment not found", zap.Any("deployment", deploymentNamespacedName))
			return "", nil
		}
		r.Logger.Error("Failed to get deployment", zap.Any("deployment", deploymentNamespacedName), zap.Error(err))
		return "", err
	}
	mode := ServeMode
	condition := depl.Status.Conditions
	if depl.Status.Replicas == 0 {
		mode = ProxyMode
	} else if depl.Status.Replicas > 0 && condition[1].Status == "True" {
		mode = ServeMode
	}

	r.Logger.Debug("Got mode from deployment", zap.Any("deployment", deploymentNamespacedName), zap.String("mode", mode))
	return mode, nil
}

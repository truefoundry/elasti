package controller

import (
	"context"

	"go.uber.org/zap"
	"truefoundry.io/elasti/api/v1alpha1"
)

func (r *ElastiServiceReconciler) serveMode(ctx context.Context, es *v1alpha1.ElastiService) error {
	r.Logger.Info("Enabling serve mode")
	privateSVCName := es.Spec.TargetSvc + "-pvt"
	privateSVC, err := r.getSVC(ctx, privateSVCName, es.Namespace)
	if err != nil {
		return err
	}
	targetSVC, err := r.getSVC(ctx, es.Spec.TargetSvc, es.Namespace)
	if err != nil {
		return err
	}

	if err = r.checkAndDeleteEendpointslices(ctx, es.Spec.TargetSvc); err != nil {
		r.Logger.Error("Failed to delete EndpointSlices", zap.Error(err))
		return err
	}
	r.Logger.Debug("Deleted EndpointSlices")
	if err = r.copySVC(ctx, targetSVC, privateSVC); err != nil {
		r.Logger.Error("Failed to copy service", zap.Error(err))
		return err
	}
	if err = r.Update(ctx, targetSVC); err != nil {
		r.Logger.Error("Failed to update target service", zap.Error(err))
		return err
	}
	r.Logger.Debug("Updated target service")
	r.Logger.Debug("Copied service")

	if err := r.Update(ctx, es); err != nil {
		r.Logger.Error("Failed to update ElastiService", zap.Error(err))
		return err
	}
	r.Logger.Debug("Updated ElastiService")
	return nil
}

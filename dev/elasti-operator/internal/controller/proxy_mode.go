package controller

import (
	"context"

	"go.uber.org/zap"
	"truefoundry.io/elasti/api/v1alpha1"
)

func (r *ElastiServiceReconciler) EnableProxyMode(ctx context.Context, es *v1alpha1.ElastiService) error {
	r.Logger.Debug("Enabling proxy mode")
	targetSVC, err := r.getSVC(ctx, es.Spec.TargetSvc, es.Namespace)
	if err != nil {
		r.Logger.Error("Failed to get target service", zap.Error(err))
		return err
	}
	r.removeSelector(ctx, targetSVC)
	r.Logger.Debug("Removed selector from target service")
	r.addTargetPort(ctx, targetSVC, 8012)
	r.Logger.Debug("Added target port to target service")

	if err = r.Update(ctx, targetSVC); err != nil {
		r.Logger.Error("Failed to update target service", zap.Error(err))
		return err
	}
	r.Logger.Debug("Updated target service")

	/*
			if targetEndpoints, err := r.getEndPoints(ctx, es.Spec.TargetSvc, es.Namespace); err != nil {
				// Check if err is not found, if yes, handle it differently
				if errors.IsNotFound(err) {
					r.Logger.Info("Target service endpoints not found", zap.String("target service", es.Spec.TargetSvc))
				} else {
					r.Logger.Error("Failed to get target service endpoints", zap.Error(err))
					return err
				}
			} else {
				r.Logger.Debug("Deleting target service endpoints", zap.Any("endpoints", targetEndpoints))
				if err = r.Delete(ctx, targetEndpoints); err != nil {
					r.Logger.Error("Failed to delete target service endpoints",
						zap.Error(err),
						zap.String("target service", es.Spec.TargetSvc),
						zap.String("namespace", es.Namespace))
					return err
				}
			}
		if err = r.checkAndDeleteEendpointslices(ctx, es.Spec.TargetSvc); err != nil {
			r.Logger.Error("Failed to delete EndpointSlices", zap.Error(err))
			return err
		}
		r.Logger.Debug("Deleted EndpointSlices")
	*/
	r.Logger.Debug("Deleted EndpointSlices")
	if err = r.copyEPS(ctx, "activator-service", es.Spec.TargetSvc, es.Namespace); err != nil {
		r.Logger.Error("Failed to copy EndpointSlices", zap.Error(err))
		return err
	}
	r.Logger.Debug("Copied EndpointSlices")

	return nil
}

package controller

import (
	"context"

	"truefoundry.io/elasti/api/v1alpha1"
)

func (r *ElastiServiceReconciler) EnableProxyMode(ctx context.Context, es *v1alpha1.ElastiService) error {
	targetSVC, err := r.getSVC(ctx, es.Spec.Service, es.Namespace)
	if err != nil {
		return err
	}
	r.removeSelector(ctx, targetSVC)
	r.addTargetPort(ctx, targetSVC, 8012)
	if err = r.Update(ctx, targetSVC); err != nil {
		return err
	}
	if err = r.createProxyEndpointSlice(ctx, targetSVC); err != nil {
		return err
	}
	return nil
}

func (r *ElastiServiceReconciler) serveMode(ctx context.Context, es *v1alpha1.ElastiService) error {
	privateSVCName := es.Spec.Service + "-pvt"
	privateSVC, err := r.getSVC(ctx, privateSVCName, es.Namespace)
	if err != nil {
		return err
	}
	if targetSVC, err := r.getSVC(ctx, es.Spec.Service, es.Namespace); err != nil {
		return err
	} else {
		if err = r.checkAndDeleteEendpointslices(ctx, es.Spec.Service); err != nil {
			return err
		}
		if err = r.copySVC(ctx, targetSVC, privateSVC); err != nil {
			return err
		}
		if err = r.Update(ctx, targetSVC); err != nil {
			return err
		}
	}
	return nil
}

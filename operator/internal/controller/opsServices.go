package controller

import (
	"context"

	"github.com/truefoundry/elasti/pkg/k8sHelper"
	"github.com/truefoundry/elasti/pkg/utils"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"truefoundry.io/elasti/api/v1alpha1"
)

func (r *ElastiServiceReconciler) deletePrivateService(ctx context.Context, publichServiceNamespacedName types.NamespacedName) (err error) {
	privateServiceNamespacedName := publichServiceNamespacedName
	privateServiceNamespacedName.Name = utils.GetPrivateSerivceName(publichServiceNamespacedName.Name)
	privateSVC := &v1.Service{}
	if err := r.Get(ctx, privateServiceNamespacedName, privateSVC); err != nil {
		if errors.IsNotFound(err) {
			r.Logger.Info("Private Service already deleted or not found", zap.String("private-service", privateServiceNamespacedName.String()))
			return nil
		}
		return err
	}
	if err := r.Delete(ctx, privateSVC); err != nil {
		return err
	}
	return nil
}

func (r *ElastiServiceReconciler) checkAndCreatePrivateService(ctx context.Context, publicSVC *v1.Service, es *v1alpha1.ElastiService) (PVTName string, err error) {
	PVTName = utils.GetPrivateSerivceName(publicSVC.Name)
	privateServiceNamespacedName := types.NamespacedName{Name: PVTName, Namespace: publicSVC.Namespace}
	// See if private service already exist
	privateSVC := &v1.Service{}
	if err := r.Get(ctx, privateServiceNamespacedName, privateSVC); err != nil && !errors.IsNotFound(err) {
		r.Logger.Error("Failed to get private service", zap.Error(err))
	} else if errors.IsNotFound(err) {
		r.Logger.Info("Private service not found, creating one", zap.String("private-service", privateServiceNamespacedName.String()))
	} else {
		r.Logger.Info("Private service already exists", zap.String("private-service", privateServiceNamespacedName.String()))
		return PVTName, nil
	}

	privateSVC = publicSVC.DeepCopy()
	privateSVC.SetName(PVTName)
	// We must remove the cluster IP and node port, as it already exists for the public service
	privateSVC.Spec.ClusterIP = ""
	privateSVC.Spec.ClusterIPs = nil
	for port := range privateSVC.Spec.Ports {
		privateSVC.Spec.Ports[port].NodePort = 0
	}
	// We also need to remove the resourceVersion
	privateSVC.ResourceVersion = ""

	// Make sure the private service is owned by the ElastiService
	if err := controllerutil.SetControllerReference(es, privateSVC, r.Scheme); err != nil {
		return PVTName, err
	}
	err = r.Create(ctx, privateSVC)
	if err != nil {
		r.Logger.Error("Failed to create private service", zap.String("private-service", privateServiceNamespacedName.String()), zap.Error(err))
		return PVTName, err
	}
	return PVTName, nil
}

func (r *ElastiServiceReconciler) handlePublicServiceChanges(ctx context.Context, obj interface{}, serviceName, namespace string) {
	publicService := &v1.Service{}
	err := k8sHelper.UnstructuredToResource(obj, publicService)
	if err != nil {
		r.Logger.Error("Failed to convert unstructured to service", zap.Error(err))
		return
	}

	if publicService.Name == serviceName {
		targetNamespacedName := types.NamespacedName{
			Name:      serviceName,
			Namespace: namespace,
		}
		targetSVC := &v1.Service{}
		if err := r.Get(ctx, targetNamespacedName, targetSVC); err != nil {
			r.Logger.Error("Failed to get service to update endpointslice", zap.String("service", targetNamespacedName.String()), zap.Error(err))
			return
		}
		if err := r.createOrUpdateEndpointsliceToResolver(ctx, targetSVC); err != nil {
			r.Logger.Error("Failed to create or update endpointslice to resolver", zap.String("service", targetNamespacedName.String()), zap.Error(err))
			return
		}
	}
	r.Logger.Info("Public service changed", zap.String("service", publicService.Name))
}

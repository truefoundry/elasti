package controller

import (
	"context"
	"fmt"

	"truefoundry/elasti/operator/api/v1alpha1"

	"github.com/truefoundry/elasti/pkg/k8sHelper"
	"github.com/truefoundry/elasti/pkg/utils"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ElastiServiceReconciler) deletePrivateService(ctx context.Context, publichServiceNamespacedName types.NamespacedName) (err error) {
	privateServiceNamespacedName := publichServiceNamespacedName
	privateServiceNamespacedName.Name = utils.GetPrivateSerivceName(publichServiceNamespacedName.Name)
	privateSVC := &v1.Service{}
	if err := r.Get(ctx, privateServiceNamespacedName, privateSVC); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get private service: %w", err)
	} else if errors.IsNotFound(err) {
		return nil
	}

	if err := r.Delete(ctx, privateSVC); err != nil {
		return fmt.Errorf("failed to delete private service: %w", err)
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

// handlePublicServiceChanges handles the changes in the public service, and sync those changes in the private service
func (r *ElastiServiceReconciler) handlePublicServiceChanges(ctx context.Context, obj interface{}, serviceName, _ string) error {
	publicSVC := &v1.Service{}
	err := k8sHelper.UnstructuredToResource(obj, publicSVC)
	if err != nil {
		return fmt.Errorf("failed to convert unstructured to service: %w", err)
	}

	// Check if the service is same as mentioned in CRD
	if publicSVC.Name != serviceName {
		return fmt.Errorf("public service is not same as mentioned in CRD; informer misconfigured")
	}
	// Get Private Service
	PVTName := utils.GetPrivateSerivceName(publicSVC.Name)
	privateServiceNamespacedName := types.NamespacedName{Name: PVTName, Namespace: publicSVC.Namespace}
	privateSVC := &v1.Service{}
	if err := r.Get(ctx, privateServiceNamespacedName, privateSVC); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get private service: %w", err)
	} else if errors.IsNotFound(err) {
		return fmt.Errorf("private service not found: %w", err)
	}

	// Sync the changes in private service
	privateSVC.Spec.Selector = publicSVC.Spec.Selector
	for port := range privateSVC.Spec.Ports {
		privateSVC.Spec.Ports[port].Name = publicSVC.Spec.Ports[port].Name
		privateSVC.Spec.Ports[port].Protocol = publicSVC.Spec.Ports[port].Protocol
		privateSVC.Spec.Ports[port].Port = publicSVC.Spec.Ports[port].Port
		privateSVC.Spec.Ports[port].TargetPort = publicSVC.Spec.Ports[port].TargetPort
		privateSVC.Spec.Ports[port].AppProtocol = publicSVC.Spec.Ports[port].AppProtocol
	}

	// Update the private service
	if err := r.Update(ctx, privateSVC); err != nil {
		return fmt.Errorf("failed to update private service: %w", err)
	}

	return nil
}

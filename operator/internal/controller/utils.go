package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"truefoundry.io/elasti/api/v1alpha1"
)

const (
	activatorServiceName = "activator-service"
	activatorPort        = 8012
)

func (r *ElastiServiceReconciler) GetModeFromDeployment(ctx context.Context, nam types.NamespacedName) (string, error) {
	depl := &appsv1.Deployment{}
	if err := r.Get(ctx, nam, depl); err != nil {
		if errors.IsNotFound(err) {
			r.Logger.Info("Deployment not found", zap.Any("namespacedName", nam))
			return "", nil
		}
		r.Logger.Error("Failed to get deployment", zap.Any("namespacedName", nam), zap.Error(err))
		return "", err
	}
	mode := ServeMode
	condition := depl.Status.Conditions
	if depl.Status.Replicas == 0 {
		mode = ProxyMode
	} else if depl.Status.Replicas > 0 && condition[1].Status == "True" {
		mode = ServeMode
	}

	r.Logger.Debug("Got mode from deployment", zap.Any("namespacedName", nam), zap.String("mode", mode))
	return mode, nil
}

func (r *ElastiServiceReconciler) DeleteEndpointsliceToActivator(ctx context.Context, namespacedName types.NamespacedName) error {
	endpointSlice := &networkingv1.EndpointSlice{}
	namespacedName.Name = namespacedName.Name + "-to-activator"
	if err := r.Get(ctx, namespacedName, endpointSlice); err != nil {
		if errors.IsNotFound(err) {
			r.Logger.Info("EndpointSlice already deleted or not found", zap.Any("namespacedName", namespacedName))
			return nil
		}
		r.Logger.Error("Failed to get endpoint slice", zap.Any("namespacedName", namespacedName), zap.Error(err))
		return err
	}
	if err := r.Delete(ctx, endpointSlice); err != nil {
		return err
	}
	return nil
}

func (r *ElastiServiceReconciler) GetIPsForActivator(ctx context.Context) ([]string, error) {
	activatorSlices := &networkingv1.EndpointSliceList{}
	if err := r.List(ctx, activatorSlices, client.MatchingLabels{
		"kubernetes.io/service-name": activatorServiceName,
	}); err != nil {
		r.Logger.Error("Failed to get activator endpoint slices", zap.Error(err))
		return nil, err
	}
	var activatorPodIPs []string
	for _, endpointSlice := range activatorSlices.Items {
		for _, endpoint := range endpointSlice.Endpoints {
			activatorPodIPs = append(activatorPodIPs, endpoint.Addresses...)
		}
	}
	if len(activatorPodIPs) == 0 {
		return nil, ErrNoActivatorPodFound
	}
	return activatorPodIPs, nil
}

func (r *ElastiServiceReconciler) CreateOrupdateEndpointsliceToActivator(ctx context.Context, service *v1.Service) error {
	activatorPodIPs, err := r.GetIPsForActivator(ctx)
	if err != nil {
		r.Logger.Error("Failed to get IPs for activator", zap.Error(err))
		return err
	}

	newEndpointsliceName := service.Name + "-to-activator"
	EndpointsliceNamespacedName := types.NamespacedName{
		Name:      newEndpointsliceName,
		Namespace: service.Namespace,
	}

	isActivatorSliceFound := false
	sliceToActivator := &networkingv1.EndpointSlice{}
	if err := r.Get(ctx, EndpointsliceNamespacedName, sliceToActivator); err != nil && !errors.IsNotFound(err) {
		r.Logger.Debug("Error getting a endpoint slice to activator", zap.Error(err))
		return err
	} else if errors.IsNotFound(err) {
		isActivatorSliceFound = false
		r.Logger.Debug("EndpointSlice not found, will try creating one", zap.Any("namespacedName", EndpointsliceNamespacedName))
	} else {
		isActivatorSliceFound = true
		r.Logger.Debug("EndpointSlice Found", zap.Any("namespacedName", EndpointsliceNamespacedName))
	}

	newEndpointSlice := &networkingv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      newEndpointsliceName,
			Namespace: service.Namespace,
			Labels: map[string]string{
				"kubernetes.io/service-name": service.Name,
			},
		},
		AddressType: networkingv1.AddressTypeIPv4,
		Ports: []networkingv1.EndpointPort{
			{
				Name:     ptr.To(service.Spec.Ports[0].Name),
				Protocol: ptr.To(v1.ProtocolTCP),
				Port:     ptr.To(int32(activatorPort)),
			},
		},
	}
	for _, ip := range activatorPodIPs {
		newEndpointSlice.Endpoints = append(newEndpointSlice.Endpoints, networkingv1.Endpoint{
			Addresses: []string{ip},
		})
	}

	if isActivatorSliceFound {
		if err := r.Update(ctx, newEndpointSlice); err != nil {
			r.Logger.Error("failed to update sliceToActivator", zap.Any("namespacedName", EndpointsliceNamespacedName), zap.Error(err))
			return err
		}
	} else {
		if err := r.Create(ctx, newEndpointSlice); err != nil {
			r.Logger.Error("failed to create sliceToActivator", zap.Any("namespacedName", EndpointsliceNamespacedName), zap.Error(err))
			return err
		}
	}

	return nil
}

func (r *ElastiServiceReconciler) CheckAndCreatePrivateService(ctx context.Context, publicSVC *v1.Service, es *v1alpha1.ElastiService) (PVTName string, err error) {
	PVTName = r.getPrivateSerivceName(publicSVC.Name)

	// See if private service already exist
	privateSVC := &v1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Name: PVTName, Namespace: publicSVC.Namespace}, privateSVC); err != nil && !errors.IsNotFound(err) {
		r.Logger.Error("Failed to get private service", zap.Error(err))
	} else if errors.IsNotFound(err) {
		r.Logger.Info("Private service not found, creating one", zap.String("name", PVTName))
	} else {
		r.Logger.Info("Private service already exists", zap.String("name", PVTName))
		return PVTName, nil
	}

	ports := []v1.ServicePort{}

	for _, port := range publicSVC.Spec.Ports {
		ports = append(ports, v1.ServicePort{
			Name:       port.Name,
			Protocol:   port.Protocol,
			Port:       port.Port,
			TargetPort: port.TargetPort,
		})
	}

	privateSVC = &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PVTName,
			Namespace: publicSVC.Namespace,
		},
		Spec: v1.ServiceSpec{
			Selector: publicSVC.Spec.Selector,
			Ports:    ports,
			Type:     v1.ServiceTypeClusterIP,
		},
	}

	if err := controllerutil.SetControllerReference(es, privateSVC, r.Scheme); err != nil {
		return PVTName, err
	}
	err = r.Create(ctx, privateSVC)
	if err != nil {
		return PVTName, err
	}
	return PVTName, nil
}

func (r *ElastiServiceReconciler) DeletePrivateService(ctx context.Context, namespacedName types.NamespacedName) (err error) {
	namespacedName.Name = r.getPrivateSerivceName(namespacedName.Name)
	privateSVC := &v1.Service{}
	if err := r.Get(ctx, namespacedName, privateSVC); err != nil {
		if errors.IsNotFound(err) {
			r.Logger.Info("Private Service already deleted or not found", zap.Any("namespacedName", namespacedName))
			return nil
		}
		return err
	}
	if err := r.Delete(ctx, privateSVC); err != nil {
		return err
	}
	return nil
}

func (r *ElastiServiceReconciler) GetES(ctx context.Context, namespacedName types.NamespacedName) (*v1alpha1.ElastiService, error) {
	es := &v1alpha1.ElastiService{}
	if err := r.Get(ctx, namespacedName, es); err != nil {
		return nil, err
	}
	return es, nil
}

func (r *ElastiServiceReconciler) UpdateESStatus(ctx context.Context, namespacedName types.NamespacedName, mode string) {
	es := &v1alpha1.ElastiService{}
	if err := r.Client.Get(ctx, namespacedName, es); err != nil {
		r.Logger.Error("Failed to get ElastiService for status update", zap.Error(err), zap.Any("namespacedName", namespacedName))
		return
	}
	es.Status.LastReconciledTime = metav1.Now()
	es.Status.Mode = mode
	if err := r.Status().Update(ctx, es); err != nil {
		r.Logger.Error("Failed to update status", zap.Error(err))
		return
	}
	r.Logger.Info("CRD Status updated successfully")
}

func (r *ElastiServiceReconciler) getPrivateSerivceName(publicSVCName string) string {
	hash := sha256.New()
	hash.Write([]byte(publicSVCName))
	hashed := hex.EncodeToString(hash.Sum(nil))
	return publicSVCName + "-" + string(hashed)[:8] + "-pvt"
}

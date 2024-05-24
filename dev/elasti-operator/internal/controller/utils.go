package controller

import (
	"context"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// checkAndDeleteEendpointslices checks and deletes the EndpointSlices for the given service
func (r *ElastiServiceReconciler) checkAndDeleteEendpointslices(ctx context.Context, service string) error {
	endpointSlicesList := &networkingv1.EndpointSliceList{}
	if err := r.List(ctx, endpointSlicesList, client.MatchingLabels{
		"kubernetes.io/service-name": service,
	}); err != nil {
		return err
	}

	for _, endpointSlice := range endpointSlicesList.Items {
		err := r.Delete(ctx, &endpointSlice)
		if err != nil {
			if errors.IsNotFound(err) {
				r.Logger.Info("EndpointSlice already deleted")
			} else {
				return err
			}
		}
	}
	return nil
}

func (r *ElastiServiceReconciler) addTargetPort(_ context.Context, service *corev1.Service, targetPort int) {
	for i := range service.Spec.Ports {
		service.Spec.Ports[i].TargetPort = intstr.FromInt(targetPort)
	}
}

func (r *ElastiServiceReconciler) removeSelector(_ context.Context, service *corev1.Service) {
	service.Spec.Selector = nil
}

// copySVC copies the service from the source service to the destination service
func (r *ElastiServiceReconciler) copySVC(_ context.Context, destSVC, sourceSVC *corev1.Service) error {
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// TODO: Reset the destSvc ports, and map the sourceSvc ports to destSvc ports
		destSVC.Spec.Ports[0].TargetPort = sourceSVC.Spec.Ports[0].TargetPort
		destSVC.Spec.Selector = sourceSVC.Spec.Selector
		return nil
	})
	if retryErr != nil {
		r.Logger.Error("Failed to update destination service", zap.String("destination service", destSVC.Name), zap.Error(retryErr))
		return retryErr
	}
	return nil
}

// createProxyEndpointSlice copies the EndpointSlices from the copyFromService to the copyToService
func (r *ElastiServiceReconciler) createProxyEndpointSlice(ctx context.Context, service *v1.Service) error {
	newEndpointsliceName := service.Name + "-to-activator"
	activatorSlices := &networkingv1.EndpointSliceList{}
	if err := r.List(ctx, activatorSlices, client.MatchingLabels{
		"kubernetes.io/service-name": "activator-service",
	}); err != nil {
		return err
	}
	var activatorPodIPs []string
	for _, endpointSlice := range activatorSlices.Items {
		for _, endpoint := range endpointSlice.Endpoints {
			activatorPodIPs = append(activatorPodIPs, endpoint.Addresses...)
		}
	}
	if len(activatorPodIPs) == 0 {
		return ErrNoActivatorPodFound
	}

	found := false
	serviceEndpointSlices, err := r.getEndpointslices(ctx, service.Name)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		r.Logger.Debug("No EndpointSlices found for the service")
	}

	if len(serviceEndpointSlices.Items) > 0 {
		for _, endpointSlice := range serviceEndpointSlices.Items {
			if endpointSlice.Name == newEndpointsliceName {
				found = true
				continue
			}
		}
	} else {
		r.Logger.Debug("Target Service Endpointslice list EMPTY")
	}

	if !found {
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
					Protocol: ptr.To(corev1.ProtocolTCP),
					Port:     ptr.To(int32(8012)),
				},
			},
		}
		for _, ip := range activatorPodIPs {
			newEndpointSlice.Endpoints = append(newEndpointSlice.Endpoints, networkingv1.Endpoint{
				Addresses: []string{ip},
			})
		}
		if err = r.Create(ctx, newEndpointSlice); err != nil {
			return err
		}
	}

	return nil
}

func (r *ElastiServiceReconciler) getSVC(ctx context.Context, svcName, namespace string) (*corev1.Service, error) {
	service := &corev1.Service{}
	if err := r.Get(ctx, client.ObjectKey{Name: svcName, Namespace: namespace}, service); err != nil {
		r.Logger.Error("Failed to get service", zap.Error(err), zap.String("service", svcName))
		return nil, err
	}
	return service, nil
}

func (r *ElastiServiceReconciler) getEndpointslices(ctx context.Context, svcName string) (*networkingv1.EndpointSliceList, error) {
	endpointSlices := &networkingv1.EndpointSliceList{}
	if err := r.List(ctx, endpointSlices, client.MatchingLabels{
		"kubernetes.io/service-name": svcName,
	}); err != nil {
		return nil, err
	}
	return endpointSlices, nil
}

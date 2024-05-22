package controller

import (
	"context"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

// copyEPS copies the EndpointSlices from the copyFromService to the copyToService
func (r *ElastiServiceReconciler) copyEPS(ctx context.Context, copyFromService, copyToService, namespace string) error {
	copyFromSlices := &networkingv1.EndpointSliceList{}
	err := r.List(ctx, copyFromSlices, client.MatchingLabels{
		"kubernetes.io/service-name": copyFromService,
	})
	if err != nil {
		r.Logger.Error("Failed to list EndpointSlices for copyFrom service", zap.Error(err))
		return err
	}
	// Collect IP addresses from activator EndpointSlices
	var podIPs []string
	for _, endpointSlice := range copyFromSlices.Items {
		for _, endpoint := range endpointSlice.Endpoints {
			podIPs = append(podIPs, endpoint.Addresses...)
		}
	}

	if len(podIPs) == 0 {
		r.Logger.Info("No pod IPs found in activator EndpointSlices")
		return nil
	}

	// TODO: We can add a check here to see if the endpointslice already exists

	// Create the new EndpointSlice for the target service
	newEndpointSlice := &networkingv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      copyToService + "-elasti-slice",
			Namespace: namespace,
			Labels: map[string]string{
				"kubernetes.io/service-name": copyToService,
			},
		},
		AddressType: networkingv1.AddressTypeIPv4,
		Ports: []networkingv1.EndpointPort{
			{
				Name:     ptr.To("http-activator-elasti"),
				Protocol: ptr.To(corev1.ProtocolTCP),
				Port:     ptr.To(int32(8012)),
			},
		},
	}

	for _, ip := range podIPs {
		newEndpointSlice.Endpoints = append(newEndpointSlice.Endpoints, networkingv1.Endpoint{
			Addresses: []string{ip},
			Conditions: networkingv1.EndpointConditions{
				Ready:       ptr.To(true),
				Serving:     ptr.To(true),
				Terminating: ptr.To(false),
			},
		})
	}

	err = r.Create(ctx, newEndpointSlice)
	if err != nil {
		r.Logger.Error("Failed to create new EndpointSlice", zap.Error(err))
		return err
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

func (r *ElastiServiceReconciler) getEndPoints(ctx context.Context, svcName, namespace string) (*corev1.Endpoints, error) {
	endpoints := &corev1.Endpoints{}
	if err := r.Get(ctx, types.NamespacedName{Name: svcName, Namespace: namespace}, endpoints); err != nil {
		r.Logger.Error("Failed to get target service", zap.Error(err))
		return endpoints, err
	}
	return endpoints, nil
}

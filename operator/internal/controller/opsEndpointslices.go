package controller

import (
	"context"
	"strconv"

	"github.com/truefoundry/elasti/pkg/utils"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// func (r *ElastiServiceReconciler) getIPsForResolver(ctx context.Context) ([]string, error) {
// 	resolverSlices := &networkingv1.EndpointSliceList{}
// 	if err := r.List(ctx, resolverSlices, client.MatchingLabels{
// 		"kubernetes.io/service-name": resolverServiceName,
// 	}); err != nil {
// 		r.Logger.Error("Failed to get Resolver endpoint slices", zap.Error(err))
// 		return nil, err
// 	}
// 	var resolverPodIPs []string
// 	for _, endpointSlice := range resolverSlices.Items {
// 		for _, endpoint := range endpointSlice.Endpoints {
// 			resolverPodIPs = append(resolverPodIPs, endpoint.Addresses...)
// 		}
// 	}
// 	if len(resolverPodIPs) == 0 {
// 		return nil, ErrNoResolverPodFound
// 	}
// 	return resolverPodIPs, nil
// }

func (r *ElastiServiceReconciler) getResolverEndpointSliceList(ctx context.Context) (*networkingv1.EndpointSliceList, error) {
	resolverSlices := &networkingv1.EndpointSliceList{}
	if err := r.List(ctx, resolverSlices, client.MatchingLabels{
		"kubernetes.io/service-name": resolverServiceName,
	}); err != nil {
		r.Logger.Error("Failed to get Resolver endpoint slices", zap.Error(err))
		return nil, err
	}
	if len(resolverSlices.Items) == 0 {
		return nil, ErrNoResolverPodFound
	}
	return resolverSlices, nil
}

func (r *ElastiServiceReconciler) deleteEndpointsliceToResolver(ctx context.Context, serviceNamespacedName types.NamespacedName) error {
	endpointSlice := &networkingv1.EndpointSlice{}
	serviceNamespacedName.Name = utils.GetEndpointSliceToResolverName(serviceNamespacedName.Name)
	if err := r.Get(ctx, serviceNamespacedName, endpointSlice); err != nil {
		if errors.IsNotFound(err) {
			r.Logger.Info("EndpointSlice already deleted or not found", zap.String("service", serviceNamespacedName.String()))
			return nil
		}
		r.Logger.Error("Failed to get endpoint slice", zap.String("service", serviceNamespacedName.String()), zap.Error(err))
		return err
	}
	if err := r.Delete(ctx, endpointSlice); err != nil {
		return err
	}
	return nil
}

func (r *ElastiServiceReconciler) createOrUpdateEndpointsliceToResolver(ctx context.Context, service *v1.Service) error {
	// resolverPodIPs, err := r.getIPsForResolver(ctx)
	// if err != nil {
	// 	r.Logger.Error("Failed to get IPs for Resolver", zap.String("service", service.Name), zap.Error(err))
	// 	return err
	// }

	// newEndpointSlice := &networkingv1.EndpointSlice{
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Name:      newEndpointsliceToResolverName,
	// 		Namespace: service.Namespace,
	// 		Labels: map[string]string{
	// 			"kubernetes.io/service-name": service.Name,
	// 		},
	// 	},
	// 	AddressType: networkingv1.AddressTypeIPv4,
	// 	Ports: []networkingv1.EndpointPort{
	// 		{
	// 			Name:     ptr.To(service.Spec.Ports[0].Name),
	// 			Protocol: ptr.To(v1.ProtocolTCP),
	// 			Port:     ptr.To(int32(resolverPort)),
	// 		},
	// 	},
	// }

	// for _, ip := range resolverPodIPs {
	// 	newEndpointSlice.Endpoints = append(newEndpointSlice.Endpoints, networkingv1.Endpoint{
	// 		Addresses: []string{ip},
	// 	})
	// }

	resolverEndpointSlicelist, err := r.getResolverEndpointSliceList(ctx)
	if err != nil {
		r.Logger.Error("Failed to get Resolver endpoint slices", zap.Error(err))
		return err
	}

	n := 0
	for _, resolverEndpointSlice := range resolverEndpointSlicelist.Items {
		newEndpointsliceToResolverName := utils.GetEndpointSliceToResolverName(service.Name) + "-" + strconv.Itoa(n)
		EndpointsliceNamespacedName := types.NamespacedName{
			Name:      newEndpointsliceToResolverName,
			Namespace: service.Namespace,
		}

		isResolverSliceFound := false
		sliceToResolver := &networkingv1.EndpointSlice{}
		if err := r.Get(ctx, EndpointsliceNamespacedName, sliceToResolver); err != nil && !errors.IsNotFound(err) {
			r.Logger.Debug("Error getting a endpoint slice to Resolver", zap.String("endpointslice", EndpointsliceNamespacedName.String()), zap.Error(err))
			return err
		} else if errors.IsNotFound(err) {
			// TODO: This can be handled better
			// This is a similar case as seen in resolver informer
			// We can handler this with the same logic as that
			isResolverSliceFound = false
			r.Logger.Debug("EndpointSlice not found, will try creating one", zap.String("endpointslice", EndpointsliceNamespacedName.String()))
		} else {
			isResolverSliceFound = true
			r.Logger.Debug("EndpointSlice Found", zap.String("endpointslice", EndpointsliceNamespacedName.String()))
		}

		sliceToResolver = resolverEndpointSlice.DeepCopy()
		// Change the service name label to the destination service
		sliceToResolver.Name = newEndpointsliceToResolverName
		sliceToResolver.Namespace = service.Namespace
		sliceToResolver.Labels["kubernetes.io/service-name"] = service.Name
		sliceToResolver.Labels["elasti.io/owner"] = "elastiservice"
		// Remove metadata that should not be copied
		sliceToResolver.ResourceVersion = ""
		sliceToResolver.UID = ""

		if isResolverSliceFound {
			if err := r.Update(ctx, sliceToResolver); err != nil {
				r.Logger.Error("failed to update sliceToResolver", zap.String("endpointslice", EndpointsliceNamespacedName.String()), zap.Error(err))
				return err
			}
			r.Logger.Info("EndpointSlice updated successfully", zap.String("endpointslice", EndpointsliceNamespacedName.String()))
		} else {
			// TODO: Make sure the private service is owned by the ElastiService
			if err := r.Create(ctx, sliceToResolver); err != nil {
				r.Logger.Error("failed to create sliceToResolver", zap.String("endpointslice", EndpointsliceNamespacedName.String()), zap.Error(err))
				return err
			}
			r.Logger.Info("EndpointSlice created successfully", zap.String("endpointslice", EndpointsliceNamespacedName.String()))
		}
		n++
	}

	return nil
}

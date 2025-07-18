package k8shelper

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Ops help you do various operations in your kubernetes cluster
type Ops struct {
	kClient *kubernetes.Clientset
	logger  *zap.Logger
}

// NewOps create a new instance for the k8s Operations
func NewOps(logger *zap.Logger, client *kubernetes.Clientset) *Ops {
	return &Ops{
		logger:  logger.Named("k8sOps"),
		kClient: client,
	}
}

// CheckIfServiceEndpointSliceActive returns true if endpoint for a service is active
func (k *Ops) CheckIfServiceEndpointSliceActive(ns, svc string) (bool, error) {
	endpointSlices, err := k.kClient.DiscoveryV1().EndpointSlices(ns).List(context.TODO(), metav1.ListOptions{
		LabelSelector: discoveryv1.LabelServiceName + "=" + svc,
	})
	if err != nil {
		return false, fmt.Errorf("CheckIfServiceEndpointSliceActive - GET: %w", err)
	}

	for _, slice := range endpointSlices.Items {
		for _, endpoint := range slice.Endpoints {
			if endpoint.Conditions.Ready != nil && *endpoint.Conditions.Ready {
				k.logger.Debug("Service endpoint is active", zap.String("service", svc), zap.String("namespace", ns))
				return true, nil
			}
		}
	}

	return false, nil
}

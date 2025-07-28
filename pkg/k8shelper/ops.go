package k8shelper

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Ops help you do various operations in your kubernetes cluster
type Ops struct {
	kClient        *kubernetes.Clientset
	kDynamicClient *dynamic.DynamicClient
	logger         *zap.Logger
}

// NewOps create a new instance for the k8s Operations
func NewOps(logger *zap.Logger, config *rest.Config) *Ops {
	kClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Fatal("Error connecting with kubernetes", zap.Error(err))
	}
	kDynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		logger.Fatal("Error connecting with kubernetes", zap.Error(err))
	}
	return &Ops{
		logger:         logger.Named("k8sOps"),
		kClient:        kClient,
		kDynamicClient: kDynamicClient,
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

	if len(endpointSlices.Items) != 1 {
		return false, fmt.Errorf("CheckIfServiceEndpointSliceActive - expected 1 slice, got %d", len(endpointSlices.Items))
	}

	if len(endpointSlices.Items[0].Endpoints) != 0 {
		isReady := endpointSlices.Items[0].Endpoints[0].Conditions.Ready

		if isReady != nil && *isReady {
			k.logger.Debug("Service endpoint is active", zap.String("service", svc), zap.String("namespace", ns))
			return true, nil
		}
	}

	return false, nil
}

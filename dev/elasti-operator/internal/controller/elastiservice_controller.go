/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"truefoundry.io/elasti/api/v1alpha1"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

// ElastiServiceReconciler reconciles a ElastiService object
type ElastiServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Logger *zap.Logger
}

//+kubebuilder:rbac:groups=elasti.truefoundry.io,resources=elastiservices,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=elasti.truefoundry.io,resources=elastiservices/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=elasti.truefoundry.io,resources=elastiservices/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ElastiService object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *ElastiServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	elastiService := &v1alpha1.ElastiService{}
	err := r.Client.Get(ctx, req.NamespacedName, elastiService)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch Pods of 'activator'
	pods := &corev1.PodList{}
	// TODO: Change the label to the one that is used in the source service, make it dynamic
	err = r.Client.List(ctx, pods, client.InNamespace("elasti"),
		client.MatchingLabels{"app": "activator"})
	if err != nil {
		r.Logger.Error("Failed to list pods",
			zap.Error(err),
			zap.Any("req", req))
		return ctrl.Result{}, err
	}

	var endpoints []discoveryv1.Endpoint
	for _, pod := range pods.Items {
		if pod.Status.PodIP != "" {
			endpoints = append(endpoints, discoveryv1.Endpoint{
				Addresses: []string{pod.Status.PodIP},
			})
		}
	}

	r.Logger.Debug("Endpoints", zap.Any("Endpoints", endpoints))

	/*
		if elastiService.Spec.Mode == "proxy" {
			// We need to get the pods of elasti-activator and add them to the target service endpointsSlice
			log.Info("Proxy Mode")

			// Code to fetch the pods of activator-service
			// Code to add the pods to the target service endpointsSlice

		} else {
			// We remove the elasti-activator pods from the target service endpointsSlice
			// We either add the selector, or we add target-service-pvt endpoints to it
			log.Info("Serve Mode")
		}
	*/

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ElastiServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ElastiService{}).
		Owns(&discoveryv1.EndpointSlice{}).
		Complete(r)
}

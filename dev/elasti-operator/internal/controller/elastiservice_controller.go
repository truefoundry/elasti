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
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	if elastiService.Spec.Mode == "proxy" {
		r.copyEndpointSlices(ctx, "activator-service", elastiService.Spec.TargetService)

		elastiService.Status.LastReconciledTime = metav1.Now()
		err = r.Status().Update(ctx, elastiService)
		if err != nil {
			r.Logger.Error("Failed to update EndpointsliceReplacer status", zap.Error(err))
			return ctrl.Result{}, err
		}
		r.Logger.Info("[Proxy Mode] Updated ElastiService status",
			zap.String("Name", elastiService.Name),
			zap.String("Namespace", elastiService.Namespace),
			zap.String("CopyFrom", "activator-service"),
			zap.String("CopyTo", elastiService.Spec.TargetService))
	} else {
		r.copyEndpointSlices(ctx, elastiService.Spec.TargetService+"-pvt", elastiService.Spec.TargetService)

		elastiService.Status.LastReconciledTime = metav1.Now()
		err = r.Status().Update(ctx, elastiService)
		if err != nil {
			r.Logger.Error("Failed to update EndpointsliceReplacer status", zap.Error(err))
			return ctrl.Result{}, err
		}
		r.Logger.Info("[Serve Mode] Updated ElastiService status",
			zap.String("Name", elastiService.Name),
			zap.String("Namespace", elastiService.Namespace),
			zap.String("CopyFrom", elastiService.Spec.TargetService+"-pvt"),
			zap.String("CopyTo", elastiService.Spec.TargetService))
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ElastiServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ElastiService{}).
		Owns(&discoveryv1.EndpointSlice{}).
		Complete(r)
}

func (r *ElastiServiceReconciler) copyEndpointSlices(ctx context.Context, copyFromService, copyToService string) error {
	copyFromSlices := &networkingv1.EndpointSliceList{}
	err := r.List(ctx, copyFromSlices, client.MatchingLabels{"kubernetes.io/service-name": copyFromService})
	if err != nil {
		r.Logger.Error("Failed to list EndpointSlices for copyFrom service", zap.Error(err))
		return err
	}
	copyToSlices := &networkingv1.EndpointSliceList{}
	err = r.List(ctx, copyToSlices, client.MatchingLabels{"kubernetes.io/service-name": copyToService})
	if err != nil {
		r.Logger.Error("Failed to list EndpointSlices for copyTo service", zap.Error(err))
		return err
	}
	// Delete the existing EndpointSlices for the target service
	/*
		for _, targetSlice := range copyToSlices.Items {
			err = r.Delete(ctx, &targetSlice)
			if err != nil {
				r.Logger.Error("Failed to delete copyToService EndpointSlice", zap.Error(err))
				return err
			}
		}
		for _, copyFromSlice := range copyFromSlices.Items {
			newSlice := copyFromSlice.DeepCopy()
			newSlice.ObjectMeta = metav1.ObjectMeta{
				Name:      copyToService,
				Namespace: copyToSlices.Items[0].Namespace,
				Labels:    map[string]string{"kubernetes.io/service-name": copyToService},
			}
			newSlice.ResourceVersion = ""
			newSlice.UID = ""
			newSlice.CreationTimestamp = metav1.Time{}
			err = r.Create(ctx, newSlice)
			if err != nil {
				r.Logger.Error("Failed to create new EndpointSlice for target service", zap.Error(err))
				return err
			}
		} */

	if len(copyFromSlices.Items) > 0 {
		// Since we are taking only the first EndpointSlice, we have a limit of 1000 endpoints
		activatorEndpoints := copyFromSlices.Items[0].Endpoints
		for i := range copyToSlices.Items {
			copyToSlices.Items[i].Endpoints = activatorEndpoints
			err = r.Update(ctx, &copyToSlices.Items[i])
			if err != nil {
				r.Logger.Error("Failed to update target EndpointSlice", zap.Error(err))
				return err
			}
		}
	}
	return nil
}

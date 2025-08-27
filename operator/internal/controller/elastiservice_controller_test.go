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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	elastiv1alpha1 "github.com/truefoundry/elasti/operator/api/v1alpha1"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("ElastiService Controller", func() {
	Context("When reconciling an elastiservice", func() {
		const (
			resourceName = "test-elasti-service"
			namespace    = "elasti-test"
		)

		ctx := context.Background()

		namespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: namespace,
		}
		elastiservice := &elastiv1alpha1.ElastiService{}
		deployment := &v1.Deployment{}
		service := &corev1.Service{}

		BeforeEach(func() {
			By("creating a new Deployment")
			deployment = &v1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
				},
				Spec: v1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": resourceName,
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": resourceName,
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, deployment)).To(Succeed())

			By("creating a new Service")
			service = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"app": resourceName,
					},
					Ports: []corev1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt32(80),
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, service)).To(Succeed())

			By("creating a new ElastiService resource")
			elastiservice = &elastiv1alpha1.ElastiService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
				},
				Spec: elastiv1alpha1.ElastiServiceSpec{
					MinTargetReplicas: 1,
					Service:           resourceName,
					ScaleTargetRef: elastiv1alpha1.ScaleTargetRef{
						APIVersion: "apps/v1",
						Kind:       "deployments",
						Name:       resourceName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, elastiservice)).To(Succeed())

			By("verifying all resources are created")
			Expect(k8sClient.Get(ctx, namespacedName, elastiservice)).To(Succeed())
			Expect(k8sClient.Get(ctx, namespacedName, deployment)).To(Succeed())
			Expect(k8sClient.Get(ctx, namespacedName, service)).To(Succeed())

		})

		AfterEach(func() {
			By("Cleaning up all resources")
			// Delete ElastiService
			resource := &elastiv1alpha1.ElastiService{}
			err := k8sClient.Get(ctx, namespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			} else if !errors.IsNotFound(err) {
				fmt.Println("error deleting elastiService: ", err)
				Fail("Error deleting elastiService")
			}

			// Delete Deployment
			deploymentResource := &v1.Deployment{}
			err = k8sClient.Get(ctx, namespacedName, deploymentResource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, deploymentResource)).To(Succeed())
			} else if !errors.IsNotFound(err) {
				fmt.Println("error deleting deployment: ", err)
				Fail("Error deleting deployment")
			}

			// Delete Service
			serviceResource := &corev1.Service{}
			err = k8sClient.Get(ctx, namespacedName, serviceResource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, serviceResource)).To(Succeed())
			} else if !errors.IsNotFound(err) {
				fmt.Println("error deleting service: ", err)
				Fail("Error deleting service")
			}
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			By("getting the deployment")
			Expect(k8sClient.Get(mgrCtx, namespacedName, deployment)).To(Succeed())

			By("reconciling the resource")
			_, err := controllerReconciler.Reconcile(mgrCtx, reconcile.Request{
				NamespacedName: namespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

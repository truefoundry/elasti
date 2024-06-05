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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ElastiServiceFinalizer = "elasti.truefoundry.com/finalizer"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ElastiServiceSpec defines the desired state of ElastiService
type ElastiServiceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Service        string `json:"service,omitempty"`
	DeploymentName string `json:"deploymentName,omitempty"`
	// How long do playground hold the request for, before dumping the queue. Default is 60s.
	QTimout int32 `json:"queueTimeout,omitempty"`
	// Idle Period is how long should the target VirtualService be Idle(without requests), before we scale it down to 0.
	IdlePeriod int32 `json:"idlePeriod,omitempty"`
}

// ElastiServiceStatus defines the observed state of ElastiService
type ElastiServiceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	LastReconciledTime metav1.Time `json:"lastReconciledTime,omitempty"`
	State              string      `json:"state,omitempty"`
	Mode               string      `json:"mode,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ElastiService is the Schema for the elastiservices API
type ElastiService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ElastiServiceSpec   `json:"spec,omitempty"`
	Status ElastiServiceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ElastiServiceList contains a list of ElastiService
type ElastiServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ElastiService `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ElastiService{}, &ElastiServiceList{})
}

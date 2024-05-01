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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ElastiServiceSpec defines the desired state of ElastiService
type ElastiServiceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// How long do activator hold the request for, before dumping the queue. Default is 60s.
	QueueTimout int32 `json:"queueTimeout,omitempty"`
	// Idle Period is how long should the target VirtualService be Idle(without requests), before we scale it down to 0.
	IdlePeriod int32 `json:"idlePeriod,omitempty"`
	// ScaledObjectRef is to map target object you wanna scale and watch. It is a KEDA object we are refering to.
	ScaledObjectRef string `json:"scaledObjectRef,omitempty"`
}

// ElastiServiceStatus defines the observed state of ElastiService
type ElastiServiceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Conditions store the status conditions of the ElastiService, like if the services are scaled or scaling to 0 or 1, in progress.
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
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

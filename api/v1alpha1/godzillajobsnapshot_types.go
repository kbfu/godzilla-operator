/*
Copyright 2023 kbfu.

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

type ChaosStepSnapshot struct {
	Name               string            `json:"name"`
	Type               LitmusType        `json:"type"`
	Config             map[string]string `json:"config"`
	Image              string            `json:"image,omitempty"`
	ServiceAccountName string            `json:"serviceAccountName"`
	Status             JobStatus         `json:"status,omitempty"`
	FailedReason       string            `json:"failedReason,omitempty"`
}

// GodzillaJobSnapshotSpec defines the desired state of GodzillaJobSnapshot
type GodzillaJobSnapshotSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Steps [][]ChaosStepSnapshot `json:"steps"`
}

// GodzillaJobSnapshotStatus defines the observed state of GodzillaJobSnapshot
type GodzillaJobSnapshotStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	JobStatus    JobStatus `json:"jobStatus"`
	FailedReason string    `json:"failedReason,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// GodzillaJobSnapshot is the Schema for the godzillajobsnapshots API
type GodzillaJobSnapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GodzillaJobSnapshotSpec   `json:"spec,omitempty"`
	Status GodzillaJobSnapshotStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GodzillaJobSnapshotList contains a list of GodzillaJobSnapshot
type GodzillaJobSnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GodzillaJobSnapshot `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GodzillaJobSnapshot{}, &GodzillaJobSnapshotList{})
}

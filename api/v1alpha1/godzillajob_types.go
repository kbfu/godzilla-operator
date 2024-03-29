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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type JobStatus string

// pending -> running -> success -> failed
//
//				     \-> failed
//	                 \-> unknown
const (
	PendingStatus JobStatus = "pending"
	RunningStatus JobStatus = "running"
	SuccessStatus JobStatus = "success"
	FailedStatus  JobStatus = "failed"
	UnknownStatus JobStatus = "unknown"
)

type (
	LitmusType   string
	GodzillaType string
)

const (
	LitmusPodDelete              LitmusType   = "litmus-pod-delete"
	LitmusPodIoStress            LitmusType   = "litmus-pod-io-stress"
	LitmusContainerKill          LitmusType   = "litmus-container-kill"
	LitmusPodMemoryStress        LitmusType   = "litmus-pod-memory-stress"
	LitmusPodCpuStress           LitmusType   = "litmus-pod-cpu-stress"
	GodzillaPodNetworkDelay      GodzillaType = "godzilla-pod-network-delay"
	GodzillaPodNetworkCorruption GodzillaType = "godzilla-pod-network-corruption"
	GodzillaPodNetworkLoss       GodzillaType = "godzilla-pod-network-loss"
	GodzillaPodNetworkDuplicate  GodzillaType = "godzilla-pod-network-duplicate"
	GodzillaPodNetworkReorder    GodzillaType = "godzilla-pod-network-reorder"
	GodzillaPodAutoscaler        GodzillaType = "godzilla-pod-autoscaler"
	GodzillaPodDiskFill          GodzillaType = "godzilla-pod-disk-fill"
)

type ChaosStep struct {
	Name               string            `json:"name"`
	Type               string            `json:"type"`
	Config             map[string]string `json:"config"`
	Image              string            `json:"image,omitempty"`
	ServiceAccountName string            `json:"serviceAccountName,omitempty"`
}

// GodzillaJobSpec defines the desired state of GodzillaJob
type GodzillaJobSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Steps [][]ChaosStep `json:"steps"`
}

// GodzillaJobStatus defines the observed state of GodzillaJob
type GodzillaJobStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// GodzillaJob is the Schema for the godzillajobs API
type GodzillaJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GodzillaJobSpec   `json:"spec,omitempty"`
	Status GodzillaJobStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GodzillaJobList contains a list of GodzillaJob
type GodzillaJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GodzillaJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GodzillaJob{}, &GodzillaJobList{})
}

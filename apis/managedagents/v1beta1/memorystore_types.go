/*
Copyright 2026 The provider-anthropic Authors.

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// MemoryStoreParameters defines the desired state of an Anthropic MemoryStore.
// These fields map to BetaMemoryStoreNewParams / BetaMemoryStoreUpdateParams
// from the Anthropic SDK.
type MemoryStoreParameters struct {
	// Required: Name is the human-readable name for the store. Up to 255 characters.
	// The mount-path slug under /mnt/memory/ is derived from this name.
	// +optional
	// +kubebuilder:validation:MaxLength=255
	Name *string `json:"name,omitempty"`

	// Description is a free-text description of what the store contains,
	// up to 1024 characters. Included in the agent's system prompt when
	// the store is attached, so word it to be useful to the agent.
	// +optional
	Description *string `json:"description,omitempty"`

	// Metadata is arbitrary key-value tags for your own bookkeeping.
	// Up to 16 pairs; keys 1–64 characters; values up to 512 characters.
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`

	// AnthropicDeletionPolicy controls whether Crossplane calls Archive or
	// Delete on the Anthropic API when the resource is deleted in Kubernetes.
	// +optional
	// +kubebuilder:validation:Enum=Archive;Delete
	// +kubebuilder:default=Archive
	AnthropicDeletionPolicy *string `json:"anthropicDeletionPolicy,omitempty"`
}

// MemoryStoreObservation holds the observed state of an Anthropic MemoryStore
// as returned by the API. These fields are read-only.
type MemoryStoreObservation struct {
	// ID is the Anthropic-assigned identifier (a memstore_... tagged ID).
	// Also stored in the external-name annotation, which the reconciler uses
	// as the primary key.
	// +optional
	ID *string `json:"id,omitempty"`

	// Name is the observed human-readable name.
	// +optional
	Name *string `json:"name,omitempty"`

	// Description is the observed free-text description.
	// +optional
	Description *string `json:"description,omitempty"`

	// Metadata is the observed key-value metadata map.
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`

	// CreatedAt is the RFC 3339 timestamp when the memory store was created.
	// +optional
	CreatedAt *string `json:"createdAt,omitempty"`

	// UpdatedAt is the RFC 3339 timestamp of the last modification.
	// +optional
	UpdatedAt *string `json:"updatedAt,omitempty"`

	// ArchivedAt is set when the memory store has been archived.
	// +optional
	ArchivedAt *string `json:"archivedAt,omitempty"`
}

// MemoryStoreSpec defines the desired state of MemoryStore.
type MemoryStoreSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`

	// ForProvider holds the configuration the provider reconciles against the
	// Anthropic API on every loop.
	ForProvider MemoryStoreParameters `json:"forProvider"`
}

// MemoryStoreStatus defines the observed state of MemoryStore.
type MemoryStoreStatus struct {
	xpv2.ManagedResourceStatus `json:",inline"`

	// AtProvider holds the observed state as returned by the Anthropic API.
	// +optional
	AtProvider MemoryStoreObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,anthropic},shortName=memstore
//
// MemoryStore is a managed resource representing an Anthropic memory store
// (beta API): a named container for agent memories, scoped to a workspace.
type MemoryStore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MemoryStoreSpec   `json:"spec"`
	Status MemoryStoreStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
//
// MemoryStoreList contains a list of MemoryStore.
type MemoryStoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MemoryStore `json:"items"`
}

// MemoryStoreKind and MemoryStoreGroupVersionKind are used by controller setup.
var (
	MemoryStoreKind             = "MemoryStore"
	MemoryStoreGroupVersionKind = GroupVersion.WithKind(MemoryStoreKind)
)

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &MemoryStore{}, &MemoryStoreList{})
		return nil
	})
}

/*
Copyright 2025 The provider-anthropic-platform Authors.

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

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	v2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// MemoryStoreMemoryParameters defines the desired state of an Anthropic
// MemoryStoreMemory. These fields map to BetaMemoryStoreMemoryNewParams /
// BetaMemoryStoreMemoryUpdateParams from the Anthropic SDK.
type MemoryStoreMemoryParameters struct {
	// MemoryStoreID is the ID of the parent MemoryStore that holds this
	// memory. Populate directly or via MemoryStoreIDRef / MemoryStoreIDSelector.
	// Immutable after creation.
	// +crossplane:generate:reference:type=github.com/jonasz-lasut/provider-anthropic-platform/apis/beta/v1alpha1.MemoryStore
	// +crossplane:generate:reference:extractor=github.com/jonasz-lasut/provider-anthropic-platform/internal/extractors.ComputedFieldExtractor("id")
	// +optional
	MemoryStoreID *string `json:"memoryStoreId,omitempty"`

	// Reference to a MemoryStore to populate memoryStoreId.
	// +kubebuilder:validation:Optional
	MemoryStoreIDRef *xpv1.NamespacedReference `json:"memoryStoreIdRef,omitempty"`

	// Selector for a MemoryStore to populate memoryStoreId.
	// +kubebuilder:validation:Optional
	MemoryStoreIDSelector *xpv1.NamespacedSelector `json:"memoryStoreIdSelector,omitempty"`

	// Required: Path is the hierarchical path of the memory within the store, e.g.
	// "/projects/foo/notes.md". Must start with "/", be at most 1,024 bytes,
	// and contain no empty/"."/".." segments. Renaming is supported via update.
	// +optional
	// +kubebuilder:validation:MaxLength=1024
	// +kubebuilder:validation:Pattern=`^/.+`
	Path *string `json:"path,omitempty"`

	// Required: ContentSecretRef references a Secret in the MR's namespace
	// holding the UTF-8 text content of the memory at the given key. Maximum
	// 100 kB (102,400 bytes); the API rejects larger payloads.
	// +optional
	ContentSecretRef *xpv1.LocalSecretKeySelector `json:"contentSecretRef,omitempty"`
}

// MemoryStoreMemoryObservation holds the observed state of an Anthropic
// MemoryStoreMemory as returned by the API. These fields are read-only.
type MemoryStoreMemoryObservation struct {
	// ID is the Anthropic-assigned memory identifier (a mem_... tagged ID).
	// Also stored in the external-name annotation, which the reconciler uses
	// as the primary key.
	// +optional
	ID *string `json:"id,omitempty"`

	// MemoryStoreID is the parent memory store ID returned by the API.
	// +optional
	MemoryStoreID *string `json:"memoryStoreId,omitempty"`

	// MemoryVersionID is the ID of the memory_version representing this
	// memory's current content (a memver_... tagged ID).
	// +optional
	MemoryVersionID *string `json:"memoryVersionId,omitempty"`

	// ContentSha256 is the lowercase hex SHA-256 digest of the stored content.
	// +optional
	ContentSha256 *string `json:"contentSha256,omitempty"`

	// ContentSizeBytes is the size of the stored content in bytes.
	// +optional
	ContentSizeBytes *int64 `json:"contentSizeBytes,omitempty"`

	// Path is the observed hierarchical path within the store.
	// +optional
	Path *string `json:"path,omitempty"`

	// CreatedAt is the RFC 3339 timestamp when the memory was created.
	// +optional
	CreatedAt *string `json:"createdAt,omitempty"`

	// UpdatedAt is the RFC 3339 timestamp of the last modification.
	// +optional
	UpdatedAt *string `json:"updatedAt,omitempty"`
}

// MemoryStoreMemorySpec defines the desired state of MemoryStoreMemory.
type MemoryStoreMemorySpec struct {
	v2.ManagedResourceSpec `json:",inline"`

	// ForProvider holds the configuration the provider reconciles against the
	// Anthropic API on every loop.
	ForProvider MemoryStoreMemoryParameters `json:"forProvider"`
}

// MemoryStoreMemoryStatus defines the observed state of MemoryStoreMemory.
type MemoryStoreMemoryStatus struct {
	xpv1.ResourceStatus `json:",inline"`

	// AtProvider holds the observed state as returned by the Anthropic API.
	// +optional
	AtProvider MemoryStoreMemoryObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,anthropic},shortName=memmem
//
// MemoryStoreMemory is a managed resource representing a single text memory
// stored at a hierarchical path inside an Anthropic MemoryStore (beta API).
type MemoryStoreMemory struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MemoryStoreMemorySpec   `json:"spec"`
	Status MemoryStoreMemoryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
//
// MemoryStoreMemoryList contains a list of MemoryStoreMemory.
type MemoryStoreMemoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MemoryStoreMemory `json:"items"`
}

// MemoryStoreMemoryKind and MemoryStoreMemoryGroupVersionKind are used by
// controller setup.
var (
	MemoryStoreMemoryKind             = "MemoryStoreMemory"
	MemoryStoreMemoryGroupVersionKind = GroupVersion.WithKind(MemoryStoreMemoryKind)
)

func init() {
	SchemeBuilder.Register(&MemoryStoreMemory{}, &MemoryStoreMemoryList{})
}

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

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	v2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// DreamModelConfig selects the model and inference speed applied to every
// stage of the dream pipeline.
type DreamModelConfig struct {
	// Required: ID is the model identifier, e.g. "claude-opus-4-7".
	// 1–256 characters.
	// +optional
	// +kubebuilder:validation:MaxLength=256
	ID *string `json:"id,omitempty"`

	// Speed is the inference speed mode. "fast" provides significantly faster
	// output-token generation at premium pricing and is not supported by every
	// model. Defaults to "standard".
	// +optional
	// +kubebuilder:validation:Enum=standard;fast
	Speed *string `json:"speed,omitempty"`
}

// DreamInput is one source the dream reads from. The Type field selects the
// variant; supply only the fields relevant to that variant. A "memory_store"
// input reads an existing MemoryStore (never mutated); a "sessions" input reads
// a set of session transcripts.
type DreamInput struct {
	// Required: Type identifies the input variant.
	// +optional
	// +kubebuilder:validation:Enum=memory_store;sessions
	Type *string `json:"type,omitempty"`

	// MemoryStoreID is the ID of the memory store to read from (memory_store
	// type). Populate directly or via MemoryStoreIDRef / MemoryStoreIDSelector.
	// +crossplane:generate:reference:type=github.com/jonasz-lasut/provider-anthropic/apis/managedagents/v1beta1.MemoryStore
	// +crossplane:generate:reference:extractor=github.com/jonasz-lasut/provider-anthropic/internal/extractors.ComputedFieldExtractor("id")
	// +optional
	MemoryStoreID *string `json:"memoryStoreId,omitempty"`

	// Reference to a MemoryStore to populate memoryStoreId.
	// +kubebuilder:validation:Optional
	MemoryStoreIDRef *xpv1.NamespacedReference `json:"memoryStoreIdRef,omitempty"`

	// Selector for a MemoryStore to populate memoryStoreId.
	// +kubebuilder:validation:Optional
	MemoryStoreIDSelector *xpv1.NamespacedSelector `json:"memoryStoreIdSelector,omitempty"`

	// SessionIDs lists the session transcript IDs to read from (sessions type).
	// Populate directly or via SessionIDsRefs / SessionIDsSelector.
	// +crossplane:generate:reference:type=github.com/jonasz-lasut/provider-anthropic/apis/managedagents/v1beta1.Session
	// +crossplane:generate:reference:extractor=github.com/jonasz-lasut/provider-anthropic/internal/extractors.ComputedFieldExtractor("id")
	// +optional
	// +listType=set
	SessionIDs []string `json:"sessionIds,omitempty"`

	// References to Sessions used to populate sessionIds.
	// +kubebuilder:validation:Optional
	SessionIDsRefs []xpv1.NamespacedReference `json:"sessionIdsRefs,omitempty"`

	// Selector for Sessions used to populate sessionIds.
	// +kubebuilder:validation:Optional
	SessionIDsSelector *xpv1.NamespacedSelector `json:"sessionIdsSelector,omitempty"`
}

// DreamParameters defines the desired state of an Anthropic Dream.
// These fields map to BetaDreamNewParams from the Anthropic SDK.
//
// A Dream is an asynchronous memory-consolidation job. It is immutable after
// creation: the Anthropic API exposes no update endpoint, so changes to any
// forProvider field are not reconciled. See docs/overlays/dream.md.
type DreamParameters struct {
	// Required: Inputs are the sources the dream reads from — typically one
	// memory store plus a set of session transcripts. Immutable after creation.
	// +optional
	Inputs []DreamInput `json:"inputs,omitempty"`

	// Required: Model identifier and configuration applied to every pipeline
	// stage. Immutable after creation.
	// +optional
	Model *DreamModelConfig `json:"model,omitempty"`

	// Instructions guide the consolidation. Immutable after creation.
	// +optional
	Instructions *string `json:"instructions,omitempty"`
}

// DreamModelObservation is the observed model configuration.
type DreamModelObservation struct {
	// ID is the model identifier.
	// +optional
	ID *string `json:"id,omitempty"`

	// Speed is the observed inference speed mode.
	// +optional
	Speed *string `json:"speed,omitempty"`
}

// DreamInputObservation is one observed input source.
type DreamInputObservation struct {
	// Type identifies the input variant ("memory_store" or "sessions").
	// +optional
	Type *string `json:"type,omitempty"`

	// MemoryStoreID is the observed memory store ID (memory_store type).
	// +optional
	MemoryStoreID *string `json:"memoryStoreId,omitempty"`

	// SessionIDs are the observed session IDs (sessions type).
	// +optional
	SessionIDs []string `json:"sessionIds,omitempty"`
}

// DreamOutputObservation is one output memory store the dream writes into.
type DreamOutputObservation struct {
	// Type identifies the output variant ("memory_store").
	// +optional
	Type *string `json:"type,omitempty"`

	// MemoryStoreID is the ID of the memory store the dream created and wrote
	// consolidated memories into.
	// +optional
	MemoryStoreID *string `json:"memoryStoreId,omitempty"`
}

// DreamErrorObservation is the failure detail for a dream whose status is
// "failed".
type DreamErrorObservation struct {
	// Message is the human-readable failure message.
	// +optional
	Message *string `json:"message,omitempty"`

	// Type is the failure type identifier.
	// +optional
	Type *string `json:"type,omitempty"`
}

// DreamUsageObservation is the cumulative token usage across every pipeline
// stage of the dream.
type DreamUsageObservation struct {
	// CacheCreationInputTokens is the number of tokens used to create
	// prompt-cache entries.
	// +optional
	CacheCreationInputTokens *int64 `json:"cacheCreationInputTokens,omitempty"`

	// CacheReadInputTokens is the number of tokens read from the prompt cache.
	// +optional
	CacheReadInputTokens *int64 `json:"cacheReadInputTokens,omitempty"`

	// InputTokens is the number of uncached input tokens consumed.
	// +optional
	InputTokens *int64 `json:"inputTokens,omitempty"`

	// OutputTokens is the number of output tokens generated.
	// +optional
	OutputTokens *int64 `json:"outputTokens,omitempty"`
}

// DreamObservation holds the observed state of an Anthropic Dream as returned
// by the API. These fields are read-only.
type DreamObservation struct {
	// ID is the Anthropic-assigned identifier. Also stored in the external-name
	// annotation, which the reconciler uses as the primary key.
	// +optional
	ID *string `json:"id,omitempty"`

	// Status is the dream lifecycle status: "pending", "running", "completed",
	// "failed", or "canceled".
	// +optional
	Status *string `json:"status,omitempty"`

	// Instructions is the observed consolidation instruction text.
	// +optional
	Instructions *string `json:"instructions,omitempty"`

	// SessionID is the session the dream is associated with.
	// +optional
	SessionID *string `json:"sessionId,omitempty"`

	// Model is the observed model configuration.
	// +optional
	Model *DreamModelObservation `json:"model,omitempty"`

	// Inputs are the observed input sources.
	// +optional
	Inputs []DreamInputObservation `json:"inputs,omitempty"`

	// Outputs are the memory stores the dream wrote consolidated memories into.
	// +optional
	Outputs []DreamOutputObservation `json:"outputs,omitempty"`

	// Error is the failure detail, populated when status is "failed".
	// +optional
	Error *DreamErrorObservation `json:"error,omitempty"`

	// Usage is the cumulative token usage across the dream.
	// +optional
	Usage *DreamUsageObservation `json:"usage,omitempty"`

	// CreatedAt is the RFC 3339 timestamp when the dream was created.
	// +optional
	CreatedAt *string `json:"createdAt,omitempty"`

	// EndedAt is the RFC 3339 timestamp when the dream reached a terminal state.
	// +optional
	EndedAt *string `json:"endedAt,omitempty"`
}

// DreamSpec defines the desired state of Dream.
type DreamSpec struct {
	v2.ManagedResourceSpec `json:",inline"`

	// ForProvider holds the configuration the provider reconciles against the
	// Anthropic API on every loop.
	ForProvider DreamParameters `json:"forProvider"`
}

// DreamStatus defines the observed state of Dream.
type DreamStatus struct {
	xpv1.ResourceStatus `json:",inline"`

	// AtProvider holds the observed state as returned by the Anthropic API.
	// +optional
	AtProvider DreamObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.atProvider.status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,anthropic},shortName=dream
//
// Dream is a managed resource representing an Anthropic Dream — an asynchronous
// memory-consolidation job (research-preview beta API). A Dream is immutable
// after creation.
type Dream struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DreamSpec   `json:"spec"`
	Status DreamStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
//
// DreamList contains a list of Dream.
type DreamList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Dream `json:"items"`
}

// DreamKind and DreamGroupVersionKind are used by controller setup.
var (
	DreamKind             = "Dream"
	DreamGroupVersionKind = GroupVersion.WithKind(DreamKind)
)

func init() {
	SchemeBuilder.Register(&Dream{}, &DreamList{})
}

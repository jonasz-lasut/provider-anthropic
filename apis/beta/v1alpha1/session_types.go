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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	v2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// SessionResourceCheckout specifies which branch or commit to check out in a
// GitHub repository resource.
type SessionResourceCheckout struct {
	// Required: Type is "branch" or "commit".
	// +optional
	// +kubebuilder:validation:Enum=branch;commit
	Type *string `json:"type,omitempty"`

	// Name is the branch name (when type=branch).
	// +optional
	Name *string `json:"name,omitempty"`

	// Sha is the full commit SHA to check out (when type=commit).
	// +optional
	Sha *string `json:"sha,omitempty"`
}

// SessionResource describes a resource mounted into the session's container.
// The Type field selects which variant is active; only supply the fields
// relevant to that variant.
type SessionResource struct {
	// Required: Type identifies the resource variant.
	// +optional
	// +kubebuilder:validation:Enum=github_repository;file;memory_store
	Type *string `json:"type,omitempty"`

	// URL is the GitHub repository URL (github_repository type).
	// +optional
	URL *string `json:"url,omitempty"`

	// AuthorizationTokenSecretRef references a Secret in the MR's namespace
	// holding the GitHub authorization token used to clone the repository
	// (github_repository type). The token is resolved at reconcile time and
	// never stored in status.atProvider.
	// +optional
	AuthorizationTokenSecretRef *xpv1.LocalSecretKeySelector `json:"authorizationTokenSecretRef,omitempty"`

	// MountPath is where to mount the resource in the container.
	// Defaults to /workspace/<repo-name> for github_repository and
	// /mnt/session/uploads/<file_id> for file resources.
	// +optional
	MountPath *string `json:"mountPath,omitempty"`

	// Checkout specifies the branch or commit to check out (github_repository type).
	// Defaults to the repository's default branch.
	// +optional
	Checkout *SessionResourceCheckout `json:"checkout,omitempty"`

	// FileID is the ID of a previously uploaded file (file type).
	// +optional
	FileID *string `json:"fileId,omitempty"`

	// MemoryStoreID is the ID of a memory store to attach (memory_store type).
	// Populate directly or via MemoryStoreIDRef / MemoryStoreIDSelector.
	// +crossplane:generate:reference:type=github.com/jonasz-lasut/provider-anthropic/apis/beta/v1alpha1.MemoryStore
	// +crossplane:generate:reference:extractor=github.com/jonasz-lasut/provider-anthropic/internal/extractors.ComputedFieldExtractor("id")
	// +optional
	MemoryStoreID *string `json:"memoryStoreId,omitempty"`

	// Reference to a MemoryStore to populate memoryStoreId.
	// +kubebuilder:validation:Optional
	MemoryStoreIDRef *xpv1.NamespacedReference `json:"memoryStoreIdRef,omitempty"`

	// Selector for a MemoryStore to populate memoryStoreId.
	// +kubebuilder:validation:Optional
	MemoryStoreIDSelector *xpv1.NamespacedSelector `json:"memoryStoreIdSelector,omitempty"`

	// Instructions guide the agent on how to use this memory store (memory_store type).
	// Maximum 4096 characters.
	// +optional
	Instructions *string `json:"instructions,omitempty"`

	// Access is the access mode for the memory store: read_write or read_only
	// (memory_store type). Defaults to read_write.
	// +optional
	// +kubebuilder:validation:Enum=read_write;read_only
	Access *string `json:"access,omitempty"`
}

// SessionParameters defines the desired state of an Anthropic Session.
// Agent, EnvironmentID, Resources, and VaultIDs are immutable after creation
// — changes to these fields will not be reconciled.
type SessionParameters struct {
	// AgentID is the ID of the Agent that runs in this session.
	// Accepts an agent ID string (pins the latest version) or use
	// AgentVersion to pin a specific version.
	// Populate directly or via AgentIDRef / AgentIDSelector.
	// +crossplane:generate:reference:type=github.com/jonasz-lasut/provider-anthropic/apis/beta/v1alpha1.Agent
	// +crossplane:generate:reference:extractor=github.com/jonasz-lasut/provider-anthropic/internal/extractors.ComputedFieldExtractor("id")
	// +optional
	AgentID *string `json:"agentId,omitempty"`

	// Reference to an Agent to populate agentId.
	// +kubebuilder:validation:Optional
	AgentIDRef *xpv1.NamespacedReference `json:"agentIdRef,omitempty"`

	// Selector for an Agent to populate agentId.
	// +kubebuilder:validation:Optional
	AgentIDSelector *xpv1.NamespacedSelector `json:"agentIdSelector,omitempty"`

	// AgentVersion pins a specific agent version. When nil, the latest version
	// of the referenced agent is used.
	// +optional
	AgentVersion *int64 `json:"agentVersion,omitempty"`

	// EnvironmentID is the ID of the Environment that defines the container
	// configuration for this session.
	// Populate directly or via EnvironmentIDRef / EnvironmentIDSelector.
	// +crossplane:generate:reference:type=github.com/jonasz-lasut/provider-anthropic/apis/beta/v1alpha1.Environment
	// +crossplane:generate:reference:extractor=github.com/jonasz-lasut/provider-anthropic/internal/extractors.ComputedFieldExtractor("id")
	// +optional
	EnvironmentID *string `json:"environmentId,omitempty"`

	// Reference to an Environment to populate environmentId.
	// +kubebuilder:validation:Optional
	EnvironmentIDRef *xpv1.NamespacedReference `json:"environmentIdRef,omitempty"`

	// Selector for an Environment to populate environmentId.
	// +kubebuilder:validation:Optional
	EnvironmentIDSelector *xpv1.NamespacedSelector `json:"environmentIdSelector,omitempty"`

	// Title is a human-readable session title.
	// +optional
	Title *string `json:"title,omitempty"`

	// Metadata is arbitrary key-value data attached to the session.
	// Maximum 16 pairs; keys up to 64 chars, values up to 512 chars.
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`

	// Resources to mount into the session's container. Immutable after creation.
	// +optional
	Resources []SessionResource `json:"resources,omitempty"`

	// VaultIDs lists vault IDs for stored credentials the agent can use.
	// Populate directly or via VaultIDsRefs / VaultIDsSelector.
	// Immutable after creation.
	// +crossplane:generate:reference:type=github.com/jonasz-lasut/provider-anthropic/apis/beta/v1alpha1.Vault
	// +crossplane:generate:reference:extractor=github.com/jonasz-lasut/provider-anthropic/internal/extractors.ComputedFieldExtractor("id")
	// +optional
	// +listType=set
	VaultIDs []string `json:"vaultIds,omitempty"`

	// References to Vaults used to populate vaultIds.
	// +kubebuilder:validation:Optional
	VaultIDsRefs []xpv1.NamespacedReference `json:"vaultIdsRefs,omitempty"`

	// Selector for Vaults used to populate vaultIds.
	// +kubebuilder:validation:Optional
	VaultIDsSelector *xpv1.NamespacedSelector `json:"vaultIdsSelector,omitempty"`

	// AnthropicDeletionPolicy controls whether Crossplane calls Archive or
	// Delete on the Anthropic API when the resource is deleted in Kubernetes.
	// +optional
	// +kubebuilder:validation:Enum=Archive;Delete
	// +kubebuilder:default=Archive
	AnthropicDeletionPolicy *string `json:"anthropicDeletionPolicy,omitempty"`
}

// SessionObservation holds the observed state of an Anthropic Session
// as returned by the API. These fields are read-only.
type SessionObservation struct {
	// ID is the Anthropic-assigned session identifier. Also stored in the
	// external-name annotation, which the reconciler uses as the primary key.
	// +optional
	ID *string `json:"id,omitempty"`

	// Title is the observed human-readable session title.
	// +optional
	Title *string `json:"title,omitempty"`

	// Metadata is the observed key-value metadata map.
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`

	// EnvironmentID is the observed environment ID returned by the API.
	// +optional
	EnvironmentID *string `json:"environmentId,omitempty"`

	// AgentID is the ID of the agent snapshot bound to this session.
	// +optional
	AgentID *string `json:"agentId,omitempty"`

	// VaultIDs is the observed list of vault IDs attached at creation.
	// +optional
	VaultIDs []string `json:"vaultIds,omitempty"`

	// Status is the current session status.
	// +optional
	Status *string `json:"status,omitempty"`

	// CreatedAt is the RFC 3339 timestamp when the session was created.
	// +optional
	CreatedAt *string `json:"createdAt,omitempty"`

	// UpdatedAt is the RFC 3339 timestamp of the last modification.
	// +optional
	UpdatedAt *string `json:"updatedAt,omitempty"`

	// ArchivedAt is set when the session has been archived.
	// +optional
	ArchivedAt *string `json:"archivedAt,omitempty"`
}

// SessionSpec defines the desired state of Session.
type SessionSpec struct {
	v2.ManagedResourceSpec `json:",inline"`

	// ForProvider holds the configuration the provider reconciles against the
	// Anthropic API on every loop.
	ForProvider SessionParameters `json:"forProvider"`
}

// SessionStatus defines the observed state of Session.
type SessionStatus struct {
	xpv1.ResourceStatus `json:",inline"`

	// AtProvider holds the observed state as returned by the Anthropic API.
	// +optional
	AtProvider SessionObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,anthropic},shortName=asession
//
// Session is a managed resource representing an Anthropic agent session
// (beta API).
type Session struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SessionSpec   `json:"spec"`
	Status SessionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
//
// SessionList contains a list of Session.
type SessionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Session `json:"items"`
}

// SessionKind and SessionGroupVersionKind are used by controller setup.
var (
	SessionKind             = "Session"
	SessionGroupVersionKind = GroupVersion.WithKind(SessionKind)
)

func init() {
	SchemeBuilder.Register(&Session{}, &SessionList{})
}

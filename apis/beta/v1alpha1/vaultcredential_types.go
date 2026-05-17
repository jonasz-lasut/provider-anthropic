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

// VaultCredentialTokenEndpointAuth selects the token-endpoint authentication
// scheme used by an MCP OAuth refresh.
type VaultCredentialTokenEndpointAuth struct {
	// Required: Type is the auth scheme: "none", "client_secret_basic" or "client_secret_post".
	// +optional
	// +kubebuilder:validation:Enum=none;client_secret_basic;client_secret_post
	Type *string `json:"type,omitempty"`

	// ClientSecretSecretRef references a Secret in the MR's namespace
	// holding the OAuth client secret at the given key. Required for
	// client_secret_basic and client_secret_post; omit for the none variant.
	// +optional
	ClientSecretSecretRef *xpv1.LocalSecretKeySelector `json:"clientSecretSecretRef,omitempty"`
}

// VaultCredentialRefresh configures OAuth refresh-token support for an
// mcp_oauth credential.
type VaultCredentialRefresh struct {
	// Required: ClientID is the OAuth client ID.
	// +optional
	ClientID *string `json:"clientId,omitempty"`

	// Required: RefreshTokenSecretRef references a Secret in the MR's
	// namespace holding the OAuth refresh token at the given key.
	// +optional
	RefreshTokenSecretRef *xpv1.LocalSecretKeySelector `json:"refreshTokenSecretRef,omitempty"`

	// Required: TokenEndpoint is the URL used to refresh the access token.
	// +optional
	TokenEndpoint *string `json:"tokenEndpoint,omitempty"`

	// TokenEndpointAuth selects the auth scheme used at the token endpoint.
	TokenEndpointAuth VaultCredentialTokenEndpointAuth `json:"tokenEndpointAuth"`

	// Resource is an optional OAuth resource indicator.
	// +optional
	Resource *string `json:"resource,omitempty"`

	// Scope is the OAuth scope for the refresh request.
	// +optional
	Scope *string `json:"scope,omitempty"`
}

// VaultCredentialAuth describes the credential payload. Type selects the
// variant; only set the fields relevant to that variant.
type VaultCredentialAuth struct {
	// Required: Type identifies the credential variant.
	// +optional
	// +kubebuilder:validation:Enum=mcp_oauth;static_bearer
	Type *string `json:"type,omitempty"`

	// Required: MCPServerURL is the URL of the MCP server this credential authenticates
	// against. Required for both variants. Immutable after creation.
	// +optional
	MCPServerURL *string `json:"mcpServerUrl,omitempty"`

	// TokenSecretRef references a Secret in the MR's namespace holding the
	// static bearer token at the given key (static_bearer variant). Omit
	// for the mcp_oauth variant.
	// +optional
	TokenSecretRef *xpv1.LocalSecretKeySelector `json:"tokenSecretRef,omitempty"`

	// AccessTokenSecretRef references a Secret in the MR's namespace
	// holding the OAuth access token at the given key (mcp_oauth variant).
	// Omit for the static_bearer variant.
	// +optional
	AccessTokenSecretRef *xpv1.LocalSecretKeySelector `json:"accessTokenSecretRef,omitempty"`

	// ExpiresAt is the RFC 3339 timestamp at which the access token expires
	// (mcp_oauth variant).
	// +optional
	ExpiresAt *string `json:"expiresAt,omitempty"`

	// Refresh configures OAuth refresh-token support (mcp_oauth variant).
	// +optional
	Refresh *VaultCredentialRefresh `json:"refresh,omitempty"`
}

// VaultCredentialParameters defines the desired state of an Anthropic
// VaultCredential. These fields map to BetaVaultCredentialNewParams /
// BetaVaultCredentialUpdateParams from the Anthropic SDK.
type VaultCredentialParameters struct {
	// VaultID is the ID of the parent Vault that stores this credential.
	// Populate directly or via VaultIDRef / VaultIDSelector. Immutable after
	// creation.
	// +crossplane:generate:reference:type=github.com/jonasz-lasut/provider-anthropic-platform/apis/beta/v1alpha1.Vault
	// +crossplane:generate:reference:extractor=github.com/jonasz-lasut/provider-anthropic-platform/internal/extractors.ComputedFieldExtractor("id")
	// +optional
	VaultID *string `json:"vaultId,omitempty"`

	// Reference to a Vault to populate vaultId.
	// +kubebuilder:validation:Optional
	VaultIDRef *xpv1.NamespacedReference `json:"vaultIdRef,omitempty"`

	// Selector for a Vault to populate vaultId.
	// +kubebuilder:validation:Optional
	VaultIDSelector *xpv1.NamespacedSelector `json:"vaultIdSelector,omitempty"`

	// Auth describes the credential payload (mcp_oauth or static_bearer).
	Auth VaultCredentialAuth `json:"auth"`

	// DisplayName is the human-readable name for the credential. Up to 255
	// characters.
	// +optional
	// +kubebuilder:validation:MaxLength=255
	DisplayName *string `json:"displayName,omitempty"`

	// Metadata is arbitrary key-value data attached to the credential.
	// Maximum 16 pairs; keys up to 64 chars, values up to 512 chars.
	// On update this is a patch: omitted keys are preserved.
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`

	// AnthropicDeletionPolicy controls whether Crossplane calls Archive or
	// Delete on the Anthropic API when the resource is deleted in Kubernetes.
	// +optional
	// +kubebuilder:validation:Enum=Archive;Delete
	// +kubebuilder:default=Archive
	AnthropicDeletionPolicy *string `json:"anthropicDeletionPolicy,omitempty"`
}

// VaultCredentialObservation holds the observed state of an Anthropic
// VaultCredential as returned by the API. These fields are read-only.
type VaultCredentialObservation struct {
	// ID is the Anthropic-assigned credential identifier. Also stored in the
	// external-name annotation, which the reconciler uses as the primary key.
	// +optional
	ID *string `json:"id,omitempty"`

	// VaultID is the parent vault ID returned by the API.
	// +optional
	VaultID *string `json:"vaultId,omitempty"`

	// CreatedAt is the RFC 3339 timestamp when the credential was created.
	// +optional
	CreatedAt *string `json:"createdAt,omitempty"`

	// UpdatedAt is the RFC 3339 timestamp of the last modification.
	// +optional
	UpdatedAt *string `json:"updatedAt,omitempty"`

	// ArchivedAt is set when the credential has been archived.
	// +optional
	ArchivedAt *string `json:"archivedAt,omitempty"`
}

// VaultCredentialSpec defines the desired state of VaultCredential.
type VaultCredentialSpec struct {
	v2.ManagedResourceSpec `json:",inline"`

	// ForProvider holds the configuration the provider reconciles against the
	// Anthropic API on every loop.
	ForProvider VaultCredentialParameters `json:"forProvider"`
}

// VaultCredentialStatus defines the observed state of VaultCredential.
type VaultCredentialStatus struct {
	xpv1.ResourceStatus `json:",inline"`

	// AtProvider holds the observed state as returned by the Anthropic API.
	// +optional
	AtProvider VaultCredentialObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,anthropic},shortName=vaultcred
//
// VaultCredential is a managed resource representing a credential stored in
// an Anthropic Vault (beta API). The credential payload itself (token, OAuth
// access token) is write-only and never returned by the API.
type VaultCredential struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VaultCredentialSpec   `json:"spec"`
	Status VaultCredentialStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
//
// VaultCredentialList contains a list of VaultCredential.
type VaultCredentialList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VaultCredential `json:"items"`
}

// VaultCredentialKind and VaultCredentialGroupVersionKind are used by
// controller setup.
var (
	VaultCredentialKind             = "VaultCredential"
	VaultCredentialGroupVersionKind = GroupVersion.WithKind(VaultCredentialKind)
)

func init() {
	SchemeBuilder.Register(&VaultCredential{}, &VaultCredentialList{})
}

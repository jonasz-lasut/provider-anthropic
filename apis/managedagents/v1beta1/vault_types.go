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

// VaultParameters defines the desired state of an Anthropic Vault.
// These fields map to BetaVaultNewParams / BetaVaultUpdateParams from the
// Anthropic SDK.
type VaultParameters struct {
	// Required: DisplayName is the human-readable name for the vault. Up to 255 characters.
	// +optional
	// +kubebuilder:validation:MaxLength=255
	DisplayName *string `json:"displayName,omitempty"`

	// Metadata is arbitrary key-value data attached to the vault.
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

// VaultObservation holds the observed state of an Anthropic Vault as returned
// by the API. These fields are read-only.
type VaultObservation struct {
	// ID is the Anthropic-assigned vault identifier. Also stored in the
	// external-name annotation, which the reconciler uses as the primary key.
	// +optional
	ID *string `json:"id,omitempty"`

	// DisplayName is the observed human-readable vault name.
	// +optional
	DisplayName *string `json:"displayName,omitempty"`

	// Metadata is the observed key-value metadata map.
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`

	// CreatedAt is the RFC 3339 timestamp when the vault was created.
	// +optional
	CreatedAt *string `json:"createdAt,omitempty"`

	// UpdatedAt is the RFC 3339 timestamp of the last modification.
	// +optional
	UpdatedAt *string `json:"updatedAt,omitempty"`

	// ArchivedAt is set when the vault has been archived.
	// +optional
	ArchivedAt *string `json:"archivedAt,omitempty"`
}

// VaultSpec defines the desired state of Vault.
type VaultSpec struct {
	v2.ManagedResourceSpec `json:",inline"`

	// ForProvider holds the configuration the provider reconciles against the
	// Anthropic API on every loop.
	ForProvider VaultParameters `json:"forProvider"`
}

// VaultStatus defines the observed state of Vault.
type VaultStatus struct {
	xpv1.ResourceStatus `json:",inline"`

	// AtProvider holds the observed state as returned by the Anthropic API.
	// +optional
	AtProvider VaultObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,anthropic},shortName=vault
//
// Vault is a managed resource representing an Anthropic credential vault
// (beta API): a container for credentials that agents use during sessions.
type Vault struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VaultSpec   `json:"spec"`
	Status VaultStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
//
// VaultList contains a list of Vault.
type VaultList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Vault `json:"items"`
}

// VaultKind and VaultGroupVersionKind are used by controller setup.
var (
	VaultKind             = "Vault"
	VaultGroupVersionKind = GroupVersion.WithKind(VaultKind)
)

func init() {
	SchemeBuilder.Register(&Vault{}, &VaultList{})
}

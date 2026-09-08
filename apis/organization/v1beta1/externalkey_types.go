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

// ExternalKeyProviderConfig identifies the customer-managed KMS key behind an
// ExternalKey. Type selects the provider and which of the other fields apply.
// The same type serves ForProvider and AtProvider.
type ExternalKeyProviderConfig struct {
	// Required: Type of the KMS provider.
	// +optional
	// +kubebuilder:validation:Enum=aws;gcp;azure
	Type *string `json:"type,omitempty"`

	// KMSARN is the full ARN of the AWS KMS key (aws). On Claude Platform on
	// AWS it must be a single-Region key in the organization's own account.
	// +optional
	KMSARN *string `json:"kmsArn,omitempty"`

	// Region is the AWS region (aws); derived from kmsArn when omitted.
	// +optional
	Region *string `json:"region,omitempty"`

	// KeyName is the fully qualified GCP KMS key name (gcp) or the Azure Key
	// Vault key name (azure).
	// +optional
	KeyName *string `json:"keyName,omitempty"`

	// TenantID is the Azure AD tenant (azure).
	// +optional
	TenantID *string `json:"tenantId,omitempty"`

	// VaultURI is the Azure Key Vault URI (azure).
	// +optional
	VaultURI *string `json:"vaultUri,omitempty"`

	// ClientID is the Azure application client ID (azure).
	// +optional
	ClientID *string `json:"clientId,omitempty"`
}

// ExternalKeyParameters defines the desired state of an Anthropic ExternalKey.
// These fields map to BetaOrganizationExternalKeyNewParams /
// BetaOrganizationExternalKeyUpdateParams from the Anthropic SDK.
type ExternalKeyParameters struct {
	// DisplayName is the human-friendly name of the key configuration.
	// +optional
	DisplayName *string `json:"displayName,omitempty"`

	// Geo is the data residency geo; only "us" is supported.
	// +optional
	// +kubebuilder:validation:Enum=us
	Geo *string `json:"geo,omitempty"`

	// Required: ProviderConfig holds the KMS provider identity and auth
	// coordinates.
	// +optional
	ProviderConfig *ExternalKeyProviderConfig `json:"providerConfig,omitempty"`
}

// ExternalKeyObservation holds the observed state of an Anthropic ExternalKey
// as returned by the API. These fields are read-only.
type ExternalKeyObservation struct {
	// ID is the Anthropic-assigned key configuration identifier (ekey_...).
	// Also stored in the external-name annotation, which the reconciler uses
	// as the primary key.
	// +optional
	ID *string `json:"id,omitempty"`

	// DisplayName is the observed display name.
	// +optional
	DisplayName *string `json:"displayName,omitempty"`

	// Geo is the observed data residency geo.
	// +optional
	Geo *string `json:"geo,omitempty"`

	// ProviderConfig is the observed KMS provider configuration.
	// +optional
	ProviderConfig *ExternalKeyProviderConfig `json:"providerConfig,omitempty"`

	// Attachment is "attached" once a workspace uses the key, otherwise
	// "unattached".
	// +optional
	Attachment *string `json:"attachment,omitempty"`

	// CreatedAt is the RFC 3339 timestamp when the key configuration was
	// created.
	// +optional
	CreatedAt *string `json:"createdAt,omitempty"`

	// UpdatedAt is the RFC 3339 timestamp of the last modification.
	// +optional
	UpdatedAt *string `json:"updatedAt,omitempty"`
}

// ExternalKeySpec defines the desired state of ExternalKey.
type ExternalKeySpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`

	// ForProvider holds the configuration the provider reconciles against the
	// Anthropic API on every loop.
	ForProvider ExternalKeyParameters `json:"forProvider"`
}

// ExternalKeyStatus defines the observed state of ExternalKey.
type ExternalKeyStatus struct {
	xpv2.ManagedResourceStatus `json:",inline"`

	// AtProvider holds the observed state as returned by the Anthropic API.
	// +optional
	AtProvider ExternalKeyObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,anthropic},shortName=ekey
//
// ExternalKey is a managed resource representing a customer-managed
// encryption key (CMEK) configuration in an Anthropic organization (Admin
// API). CMEK must be enabled for the organization. Requires a ProviderConfig
// backed by an Admin API key.
type ExternalKey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExternalKeySpec   `json:"spec"`
	Status ExternalKeyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
//
// ExternalKeyList contains a list of ExternalKey.
type ExternalKeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExternalKey `json:"items"`
}

// ExternalKeyKind and ExternalKeyGroupVersionKind are used by controller setup.
var (
	ExternalKeyKind             = "ExternalKey"
	ExternalKeyGroupVersionKind = GroupVersion.WithKind(ExternalKeyKind)
)

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &ExternalKey{}, &ExternalKeyList{})
		return nil
	})
}

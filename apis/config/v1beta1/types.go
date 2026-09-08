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

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// ProviderConfigSpec defines the credentials for authenticating to the
// Anthropic API.
type ProviderConfigSpec struct {
	// Credentials required to authenticate to the Anthropic API. The resolved
	// credential payload must be a JSON object whose fields depend on the
	// configured identity type, e.g. {"api_key": "sk-ant-..."} for APIKey.
	Credentials ProviderCredentials `json:"credentials"`

	// Identity specifies how the provider authenticates to the Anthropic API.
	// +kubebuilder:validation:Required
	Identity *Identity `json:"identity"`

	// WorkspaceID selects the Anthropic workspace that every request made
	// through this ProviderConfig runs in, sent as the anthropic-workspace-id
	// header. Only multi-workspace personal or service-account API keys honour
	// it; a key bound to a single workspace always runs there. Accepts a
	// wrkspc_ prefixed workspace ID or the literal "default" for the
	// organization's Default Workspace. Changing it re-targets every managed
	// resource using this ProviderConfig to the new workspace, where their
	// external names do not exist.
	// +optional
	// +kubebuilder:validation:Pattern=`^(wrkspc_[A-Za-z0-9]+|default)$`
	WorkspaceID *string `json:"workspaceID,omitempty"`
}

// ProviderCredentials specifies how to obtain the Anthropic credentials.
type ProviderCredentials struct {
	// Source of the credentials.
	// +kubebuilder:validation:Enum=None;Secret;InjectedIdentity;Environment;Filesystem
	Source xpv2.CredentialsSource `json:"source"`

	xpv2.CommonCredentialSelectors `json:",inline"`
}

// IdentityType describes which authentication method the provider uses to call
// the Anthropic API.
type IdentityType string

const (
	// IdentityTypeAPIKey authenticates using a static API key read from the
	// "api_key" field of the JSON credentials payload.
	IdentityTypeAPIKey IdentityType = "APIKey"

	// IdentityTypeWorkloadIdentityFederation authenticates with short-lived
	// access tokens that the provider mints by exchanging a Kubernetes
	// projected ServiceAccount token through an Anthropic federation rule.
	// No credentials Secret is involved; set credentials.source to None.
	IdentityTypeWorkloadIdentityFederation IdentityType = "WorkloadIdentityFederation"
)

// Identity specifies the authentication identity configuration.
// +kubebuilder:validation:XValidation:rule="self.type != 'WorkloadIdentityFederation' || has(self.federation)",message="spec.identity.federation is required when type is WorkloadIdentityFederation"
type Identity struct {
	// Type of identity used to authenticate to the Anthropic API.
	// APIKey: a static API key read from the "api_key" field of the JSON
	// credentials payload (regular, organization-scoped, or Admin API key).
	// WorkloadIdentityFederation: short-lived access tokens exchanged from a
	// projected Kubernetes ServiceAccount token through the federation rule
	// in spec.identity.federation; the only identity accepted by the Admin
	// API's service-account and federation endpoints.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=APIKey;WorkloadIdentityFederation
	Type IdentityType `json:"type"`

	// Federation configures the WorkloadIdentityFederation identity. Required
	// for that type and ignored for the others.
	// +optional
	Federation *FederationIdentity `json:"federation,omitempty"`
}

// FederationIdentity describes how the provider exchanges its Kubernetes
// identity for Anthropic access tokens. The SDK performs the jwt-bearer
// exchange at /v1/oauth/token, caches the access token, and re-exchanges it
// near expiry, so nothing long-lived is ever stored.
type FederationIdentity struct {
	// OrganizationID is the UUID of the Anthropic organization the federation
	// rule belongs to (the id returned by GET /v1/organizations/me).
	// +kubebuilder:validation:MinLength=1
	OrganizationID string `json:"organizationID"`

	// FederationRuleID is the federation rule (fdrl_...) whose issuer trusts
	// the cluster's ServiceAccount tokens and whose target decides the role
	// and scope of the minted access tokens.
	// +kubebuilder:validation:Pattern=`^fdrl_[A-Za-z0-9]+$`
	FederationRuleID string `json:"federationRuleID"`

	// ServiceAccountID is an optional expected-target check (svac_...) for
	// rules that target a service account; the exchange fails if the rule
	// targets a different account.
	// +optional
	// +kubebuilder:validation:Pattern=`^svac_[A-Za-z0-9]+$`
	ServiceAccountID *string `json:"serviceAccountID,omitempty"`

	// TokenFile is the path of the identity token inside the provider pod,
	// normally a projected ServiceAccount token volume that the
	// DeploymentRuntimeConfig mounts with the audience the federation rule
	// expects; the kubelet rotates it and every exchange reads it fresh.
	// +optional
	// +kubebuilder:default="/var/run/secrets/anthropic/token"
	TokenFile *string `json:"tokenFile,omitempty"`
}

// ProviderConfigStatus represents the observed state of a ProviderConfig.
type ProviderConfigStatus struct {
	xpv2.ProviderConfigStatus `json:",inline"`
}

// +kubebuilder:object:root=true

// A ProviderConfig configures the AzureAD provider.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="SECRET-NAME",type="string",JSONPath=".spec.credentials.secretRef.name",priority=1
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,provider,anthropic}
// +kubebuilder:storageversion
type ProviderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProviderConfigSpec   `json:"spec"`
	Status ProviderConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProviderConfigList contains a list of ProviderConfig.
type ProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProviderConfig `json:"items"`
}

// +kubebuilder:object:root=true

// A ProviderConfigUsage indicates that a resource is using a ProviderConfig.
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="CONFIG-NAME",type="string",JSONPath=".providerConfigRef.name"
// +kubebuilder:printcolumn:name="RESOURCE-KIND",type="string",JSONPath=".resourceRef.kind"
// +kubebuilder:printcolumn:name="RESOURCE-NAME",type="string",JSONPath=".resourceRef.name"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,provider,anthropic}
// +kubebuilder:storageversion
type ProviderConfigUsage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	xpv2.TypedProviderConfigUsage `json:",inline"`
}

// +kubebuilder:object:root=true

// ProviderConfigUsageList contains a list of ProviderConfigUsage
type ProviderConfigUsageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProviderConfigUsage `json:"items"`
}

// +kubebuilder:object:root=true

// ClusterProviderConfig configures how the provider authenticates to the Anthropic API.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="SECRET-NAME",type="string",JSONPath=".spec.credentials.secretRef.name",priority=1
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:resource:scope=Cluster,categories={crossplane,provider,anthropic}
// +kubebuilder:storageversion
type ClusterProviderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProviderConfigSpec   `json:"spec"`
	Status ProviderConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterProviderConfigList contains a list of ProviderConfig.
type ClusterProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterProviderConfig `json:"items"`
}

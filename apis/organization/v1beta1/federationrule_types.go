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

// FederationRuleMatch lists the conditions a verified JWT must satisfy for the
// rule to apply. At least one of subjectPrefix, claims, or condition is
// required; audience alone is not sufficient. The same type serves
// ForProvider and AtProvider.
type FederationRuleMatch struct {
	// SubjectPrefix matches the sub claim exactly, or as a prefix when the
	// value ends with "*", for example
	// "system:serviceaccount:crossplane-system:provider-anthropic-*".
	// +optional
	SubjectPrefix *string `json:"subjectPrefix,omitempty"`

	// Claims are exact-match pairs against top-level string claims.
	// +optional
	Claims map[string]string `json:"claims,omitempty"`

	// Audience matches the aud claim (any element if it is an array). When
	// omitted the aud must equal Anthropic's expected audience for the
	// issuer; setting it overrides that default.
	// +optional
	Audience *string `json:"audience,omitempty"`

	// Condition is a CEL expression over the claims variable for logic the
	// structural fields cannot express; a constant-true expression is
	// rejected.
	// +optional
	Condition *string `json:"condition,omitempty"`
}

// FederationRuleParameters defines the desired state of an Anthropic
// FederationRule. These fields map to
// BetaOrganizationFederationRuleNewParams /
// BetaOrganizationFederationRuleUpdateParams from the Anthropic SDK.
type FederationRuleParameters struct {
	// Required: Name is a slug (lowercase, digits, hyphens), unique within
	// the organization; a duplicate returns 409.
	// +optional
	// +kubebuilder:validation:Pattern=`^[a-z0-9][a-z0-9-]*$`
	Name *string `json:"name,omitempty"`

	// Description is free text.
	// +optional
	Description *string `json:"description,omitempty"`

	// Required: IssuerID is the FederationIssuer whose tokens this rule
	// accepts. Populate directly or via IssuerIDRef / IssuerIDSelector.
	// +crossplane:generate:reference:type=github.com/jonasz-lasut/provider-anthropic/apis/organization/v1beta1.FederationIssuer
	// +crossplane:generate:reference:extractor=github.com/jonasz-lasut/provider-anthropic/internal/extractors.ComputedFieldExtractor("id")
	// +optional
	IssuerID *string `json:"issuerId,omitempty"`

	// Reference to a FederationIssuer to populate issuerId.
	// +kubebuilder:validation:Optional
	IssuerIDRef *xpv2.NamespacedReference `json:"issuerIdRef,omitempty"`

	// Selector for a FederationIssuer to populate issuerId.
	// +kubebuilder:validation:Optional
	IssuerIDSelector *xpv2.NamespacedSelector `json:"issuerIdSelector,omitempty"`

	// Required: ServiceAccountID is the ServiceAccount that tokens minted via
	// this rule act as. Populate directly or via ServiceAccountIDRef /
	// ServiceAccountIDSelector.
	// +crossplane:generate:reference:type=github.com/jonasz-lasut/provider-anthropic/apis/organization/v1beta1.ServiceAccount
	// +crossplane:generate:reference:extractor=github.com/jonasz-lasut/provider-anthropic/internal/extractors.ComputedFieldExtractor("id")
	// +optional
	ServiceAccountID *string `json:"serviceAccountId,omitempty"`

	// Reference to a ServiceAccount to populate serviceAccountId.
	// +kubebuilder:validation:Optional
	ServiceAccountIDRef *xpv2.NamespacedReference `json:"serviceAccountIdRef,omitempty"`

	// Selector for a ServiceAccount to populate serviceAccountId.
	// +kubebuilder:validation:Optional
	ServiceAccountIDSelector *xpv2.NamespacedSelector `json:"serviceAccountIdSelector,omitempty"`

	// Required: OAuthScope is the space-separated scopes of minted tokens.
	// Rules created through the API may grant workspace:developer or
	// workspace:inference; org:admin requires a Console session.
	// +optional
	OAuthScope *string `json:"oauthScope,omitempty"`

	// Required: Match lists the conditions the verified JWT must satisfy.
	// +optional
	Match *FederationRuleMatch `json:"match,omitempty"`

	// WorkspaceID enables the rule for one workspace. Required unless
	// AppliesToAllWorkspaces is true. Populate directly or via
	// WorkspaceIDRef / WorkspaceIDSelector.
	// +crossplane:generate:reference:type=github.com/jonasz-lasut/provider-anthropic/apis/organization/v1beta1.Workspace
	// +crossplane:generate:reference:extractor=github.com/jonasz-lasut/provider-anthropic/internal/extractors.ComputedFieldExtractor("id")
	// +optional
	WorkspaceID *string `json:"workspaceId,omitempty"`

	// Reference to a Workspace to populate workspaceId.
	// +kubebuilder:validation:Optional
	WorkspaceIDRef *xpv2.NamespacedReference `json:"workspaceIdRef,omitempty"`

	// Selector for a Workspace to populate workspaceId.
	// +kubebuilder:validation:Optional
	WorkspaceIDSelector *xpv2.NamespacedSelector `json:"workspaceIdSelector,omitempty"`

	// AppliesToAllWorkspaces enables the rule for every workspace, including
	// ones created later.
	// +optional
	AppliesToAllWorkspaces *bool `json:"appliesToAllWorkspaces,omitempty"`

	// TokenLifetimeSeconds is the lifetime of minted access tokens (60 to
	// 86400). The API default is 3600.
	// +optional
	// +kubebuilder:validation:Minimum=60
	// +kubebuilder:validation:Maximum=86400
	TokenLifetimeSeconds *int64 `json:"tokenLifetimeSeconds,omitempty"`
}

// FederationRuleObservation holds the observed state of an Anthropic
// FederationRule as returned by the API. These fields are read-only.
type FederationRuleObservation struct {
	// ID is the Anthropic-assigned rule identifier (fdrl_...). Also stored
	// in the external-name annotation, which the reconciler uses as the
	// primary key.
	// +optional
	ID *string `json:"id,omitempty"`

	// Name is the observed slug.
	// +optional
	Name *string `json:"name,omitempty"`

	// Description is the observed description.
	// +optional
	Description *string `json:"description,omitempty"`

	// IssuerID is the observed issuer.
	// +optional
	IssuerID *string `json:"issuerId,omitempty"`

	// IssuerName is the observed issuer slug.
	// +optional
	IssuerName *string `json:"issuerName,omitempty"`

	// ServiceAccountID is the observed target service account.
	// +optional
	ServiceAccountID *string `json:"serviceAccountId,omitempty"`

	// ServiceAccountName is the observed target service account name.
	// +optional
	ServiceAccountName *string `json:"serviceAccountName,omitempty"`

	// OAuthScope is the observed scope of minted tokens.
	// +optional
	OAuthScope *string `json:"oauthScope,omitempty"`

	// Match is the observed match configuration.
	// +optional
	Match *FederationRuleMatch `json:"match,omitempty"`

	// WorkspaceID is the observed primary workspace binding.
	// +optional
	WorkspaceID *string `json:"workspaceId,omitempty"`

	// WorkspaceIDs lists every workspace the rule is enabled for.
	// +optional
	WorkspaceIDs []string `json:"workspaceIds,omitempty"`

	// AppliesToAllWorkspaces is the observed all-workspaces flag.
	// +optional
	AppliesToAllWorkspaces *bool `json:"appliesToAllWorkspaces,omitempty"`

	// TokenLifetimeSeconds is the observed token lifetime.
	// +optional
	TokenLifetimeSeconds *int64 `json:"tokenLifetimeSeconds,omitempty"`

	// CreatedAt is the RFC 3339 timestamp when the rule was created.
	// +optional
	CreatedAt *string `json:"createdAt,omitempty"`

	// UpdatedAt is the RFC 3339 timestamp of the last modification.
	// +optional
	UpdatedAt *string `json:"updatedAt,omitempty"`

	// CreatedByActorID identifies who created the rule.
	// +optional
	CreatedByActorID *string `json:"createdByActorId,omitempty"`

	// UpdatedByActorID identifies who last modified the rule.
	// +optional
	UpdatedByActorID *string `json:"updatedByActorId,omitempty"`

	// ArchivedAt is set when the rule has been archived.
	// +optional
	ArchivedAt *string `json:"archivedAt,omitempty"`
}

// FederationRuleSpec defines the desired state of FederationRule.
type FederationRuleSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`

	// ForProvider holds the configuration the provider reconciles against the
	// Anthropic API on every loop.
	ForProvider FederationRuleParameters `json:"forProvider"`
}

// FederationRuleStatus defines the observed state of FederationRule.
type FederationRuleStatus struct {
	xpv2.ManagedResourceStatus `json:",inline"`

	// AtProvider holds the observed state as returned by the Anthropic API.
	// +optional
	AtProvider FederationRuleObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,anthropic},shortName=fdrl
//
// FederationRule is a managed resource representing a workload identity
// federation rule in an Anthropic organization (Admin API): which issuer's
// tokens, matching which claims, may be exchanged for which service
// account's access tokens. The endpoint accepts only org:admin OAuth tokens,
// so it requires a ProviderConfig with the WorkloadIdentityFederation
// identity.
type FederationRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FederationRuleSpec   `json:"spec"`
	Status FederationRuleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
//
// FederationRuleList contains a list of FederationRule.
type FederationRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FederationRule `json:"items"`
}

// FederationRuleKind and FederationRuleGroupVersionKind are used by
// controller setup.
var (
	FederationRuleKind             = "FederationRule"
	FederationRuleGroupVersionKind = GroupVersion.WithKind(FederationRuleKind)
)

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &FederationRule{}, &FederationRuleList{})
		return nil
	})
}

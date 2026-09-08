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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// FederationIssuerJWKS configures how the signing keys of an issuer are
// obtained. Type selects the variant and which of the other fields apply. The
// same type serves ForProvider and AtProvider.
type FederationIssuerJWKS struct {
	// Type is discovery (OIDC discovery from the issuer URL, the API default),
	// explicit_url (a fixed JWKS URL), or inline (keys given here, for
	// issuers Anthropic cannot reach, such as a private Kubernetes cluster).
	// +optional
	// +kubebuilder:validation:Enum=discovery;explicit_url;inline
	Type *string `json:"type,omitempty"`

	// DiscoveryBase overrides the discovery URL when it differs from the
	// issuer URL (discovery).
	// +optional
	DiscoveryBase *string `json:"discoveryBase,omitempty"`

	// URL is the JWKS document URL (explicit_url).
	// +optional
	URL *string `json:"url,omitempty"`

	// CACertPEM is an optional custom CA for TLS verification of the JWKS
	// fetch (discovery, explicit_url).
	// +optional
	CACertPEM *string `json:"caCertPem,omitempty"`

	// Keys are the JWK objects (inline), for example the output of
	// `kubectl get --raw /openid/v1/jwks`.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Keys []apiextensionsv1.JSON `json:"keys,omitempty"`
}

// FederationIssuerPollStatus reports the issuer's JWKS polling state.
type FederationIssuerPollStatus struct {
	// ConsecutiveFailures counts failed JWKS fetches in a row.
	// +optional
	ConsecutiveFailures *int64 `json:"consecutiveFailures,omitempty"`

	// LastFetchedAt is the RFC 3339 timestamp of the last successful fetch.
	// +optional
	LastFetchedAt *string `json:"lastFetchedAt,omitempty"`

	// NextPollAt is the RFC 3339 timestamp of the next scheduled fetch.
	// +optional
	NextPollAt *string `json:"nextPollAt,omitempty"`
}

// FederationIssuerParameters defines the desired state of an Anthropic
// FederationIssuer. These fields map to
// BetaOrganizationFederationIssuerNewParams /
// BetaOrganizationFederationIssuerUpdateParams from the Anthropic SDK.
type FederationIssuerParameters struct {
	// Required: Name is a slug (lowercase, digits, hyphens), unique within
	// the organization; a duplicate returns 409.
	// +optional
	// +kubebuilder:validation:Pattern=`^[a-z0-9][a-z0-9-]*$`
	Name *string `json:"name,omitempty"`

	// Required: IssuerURL is the iss claim value tokens must carry, for
	// example https://kubernetes.default.svc.cluster.local for a Kubernetes
	// cluster.
	// +optional
	IssuerURL *string `json:"issuerUrl,omitempty"`

	// CheckJTI enforces single use of assertions that carry a jti claim. The
	// API default is true.
	// +optional
	CheckJTI *bool `json:"checkJti,omitempty"`

	// MaxJWTLifetimeSeconds bounds the iat to exp spread of assertions
	// (1 to 176400). The API default is 3600.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=176400
	MaxJWTLifetimeSeconds *int64 `json:"maxJwtLifetimeSeconds,omitempty"`

	// JWKS configures how signing keys are obtained. Defaults to OIDC
	// discovery.
	// +optional
	JWKS *FederationIssuerJWKS `json:"jwks,omitempty"`

	// JWKSPollingDisabled stops periodic JWKS re-fetching. Update only.
	// +optional
	JWKSPollingDisabled *bool `json:"jwksPollingDisabled,omitempty"`
}

// FederationIssuerObservation holds the observed state of an Anthropic
// FederationIssuer as returned by the API. These fields are read-only.
type FederationIssuerObservation struct {
	// ID is the Anthropic-assigned issuer identifier (fdis_...). Also stored
	// in the external-name annotation, which the reconciler uses as the
	// primary key.
	// +optional
	ID *string `json:"id,omitempty"`

	// Name is the observed slug.
	// +optional
	Name *string `json:"name,omitempty"`

	// IssuerURL is the observed iss value.
	// +optional
	IssuerURL *string `json:"issuerUrl,omitempty"`

	// CheckJTI is the observed replay-protection setting.
	// +optional
	CheckJTI *bool `json:"checkJti,omitempty"`

	// MaxJWTLifetimeSeconds is the observed assertion lifetime bound.
	// +optional
	MaxJWTLifetimeSeconds *int64 `json:"maxJwtLifetimeSeconds,omitempty"`

	// JWKS is the observed key source configuration.
	// +optional
	JWKS *FederationIssuerJWKS `json:"jwks,omitempty"`

	// JWKSPollingDisabledAt is set when polling has been disabled.
	// +optional
	JWKSPollingDisabledAt *string `json:"jwksPollingDisabledAt,omitempty"`

	// PollStatus reports the JWKS polling state.
	// +optional
	PollStatus *FederationIssuerPollStatus `json:"pollStatus,omitempty"`

	// CreatedAt is the RFC 3339 timestamp when the issuer was created.
	// +optional
	CreatedAt *string `json:"createdAt,omitempty"`

	// UpdatedAt is the RFC 3339 timestamp of the last modification.
	// +optional
	UpdatedAt *string `json:"updatedAt,omitempty"`

	// CreatedByActorID identifies who created the issuer.
	// +optional
	CreatedByActorID *string `json:"createdByActorId,omitempty"`

	// UpdatedByActorID identifies who last modified the issuer.
	// +optional
	UpdatedByActorID *string `json:"updatedByActorId,omitempty"`

	// ArchivedAt is set when the issuer has been archived.
	// +optional
	ArchivedAt *string `json:"archivedAt,omitempty"`
}

// FederationIssuerSpec defines the desired state of FederationIssuer.
type FederationIssuerSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`

	// ForProvider holds the configuration the provider reconciles against the
	// Anthropic API on every loop.
	ForProvider FederationIssuerParameters `json:"forProvider"`
}

// FederationIssuerStatus defines the observed state of FederationIssuer.
type FederationIssuerStatus struct {
	xpv2.ManagedResourceStatus `json:",inline"`

	// AtProvider holds the observed state as returned by the Anthropic API.
	// +optional
	AtProvider FederationIssuerObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,anthropic},shortName=fdis
//
// FederationIssuer is a managed resource representing an OIDC issuer that an
// Anthropic organization trusts for workload identity federation (Admin
// API). The endpoint accepts only org:admin OAuth tokens, so it requires a
// ProviderConfig with the WorkloadIdentityFederation identity.
type FederationIssuer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FederationIssuerSpec   `json:"spec"`
	Status FederationIssuerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
//
// FederationIssuerList contains a list of FederationIssuer.
type FederationIssuerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FederationIssuer `json:"items"`
}

// FederationIssuerKind and FederationIssuerGroupVersionKind are used by
// controller setup.
var (
	FederationIssuerKind             = "FederationIssuer"
	FederationIssuerGroupVersionKind = GroupVersion.WithKind(FederationIssuerKind)
)

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &FederationIssuer{}, &FederationIssuerList{})
		return nil
	})
}

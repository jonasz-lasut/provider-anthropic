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

// TunnelCertificateParameters defines the desired state of an Anthropic
// TunnelCertificate. These fields map to BetaTunnelCertificateNewParams from
// the Anthropic SDK.
//
// A TunnelCertificate is a sub-resource of a Tunnel: it registers a public CA
// certificate that Anthropic trusts for the tunnel. It is immutable after
// creation (the API exposes no update endpoint). See
// docs/overlays/tunnelcertificate.md.
type TunnelCertificateParameters struct {
	// TunnelID is the ID of the parent Tunnel this certificate is registered on.
	// Populate directly or via TunnelIDRef / TunnelIDSelector. Immutable after
	// creation.
	// +crossplane:generate:reference:type=github.com/jonasz-lasut/provider-anthropic/apis/managedagents/v1beta1.Tunnel
	// +crossplane:generate:reference:extractor=github.com/jonasz-lasut/provider-anthropic/internal/extractors.ComputedFieldExtractor("id")
	// +optional
	TunnelID *string `json:"tunnelId,omitempty"`

	// Reference to a Tunnel to populate tunnelId.
	// +kubebuilder:validation:Optional
	TunnelIDRef *xpv2.NamespacedReference `json:"tunnelIdRef,omitempty"`

	// Selector for a Tunnel to populate tunnelId.
	// +kubebuilder:validation:Optional
	TunnelIDSelector *xpv2.NamespacedSelector `json:"tunnelIdSelector,omitempty"`

	// Required: CaCertificatePemSecretRef references a Secret in the MR's
	// namespace holding a PEM-encoded X.509 CA certificate at the given key.
	// The referenced value must contain exactly one certificate and no
	// private-key material, and be at most 8 kB. Immutable after creation.
	// +optional
	CaCertificatePemSecretRef *xpv2.LocalSecretKeySelector `json:"caCertificatePemSecretRef,omitempty"`
}

// TunnelCertificateObservation holds the observed state of an Anthropic
// TunnelCertificate as returned by the API. These fields are read-only.
type TunnelCertificateObservation struct {
	// ID is the Anthropic-assigned identifier (a tcrt_... tagged ID). Also
	// stored in the external-name annotation, which the reconciler uses as the
	// primary key.
	// +optional
	ID *string `json:"id,omitempty"`

	// TunnelID is the parent tunnel ID returned by the API.
	// +optional
	TunnelID *string `json:"tunnelId,omitempty"`

	// Fingerprint is the lowercase hex SHA-256 fingerprint of the certificate's
	// DER encoding.
	// +optional
	Fingerprint *string `json:"fingerprint,omitempty"`

	// ExpiresAt is the RFC 3339 timestamp when the certificate expires.
	// +optional
	ExpiresAt *string `json:"expiresAt,omitempty"`

	// CreatedAt is the RFC 3339 timestamp when the certificate was registered.
	// +optional
	CreatedAt *string `json:"createdAt,omitempty"`
}

// TunnelCertificateSpec defines the desired state of TunnelCertificate.
type TunnelCertificateSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`

	// ForProvider holds the configuration the provider reconciles against the
	// Anthropic API on every loop.
	ForProvider TunnelCertificateParameters `json:"forProvider"`
}

// TunnelCertificateStatus defines the observed state of TunnelCertificate.
type TunnelCertificateStatus struct {
	xpv2.ManagedResourceStatus `json:",inline"`

	// AtProvider holds the observed state as returned by the Anthropic API.
	// +optional
	AtProvider TunnelCertificateObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="TUNNEL",type="string",JSONPath=".status.atProvider.tunnelId"
// +kubebuilder:printcolumn:name="EXPIRES",type="string",JSONPath=".status.atProvider.expiresAt"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,anthropic},shortName=tnlcert
//
// TunnelCertificate is a managed resource representing a public CA certificate
// registered on an Anthropic MCP tunnel (research-preview beta API). It is
// immutable after creation.
type TunnelCertificate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TunnelCertificateSpec   `json:"spec"`
	Status TunnelCertificateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
//
// TunnelCertificateList contains a list of TunnelCertificate.
type TunnelCertificateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TunnelCertificate `json:"items"`
}

// TunnelCertificateKind and TunnelCertificateGroupVersionKind are used by
// controller setup.
var (
	TunnelCertificateKind             = "TunnelCertificate"
	TunnelCertificateGroupVersionKind = GroupVersion.WithKind(TunnelCertificateKind)
)

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &TunnelCertificate{}, &TunnelCertificateList{})
		return nil
	})
}

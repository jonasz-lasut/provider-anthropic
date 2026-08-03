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

// TunnelParameters defines the desired state of an Anthropic Tunnel. These
// fields map to BetaTunnelNewParams from the Anthropic SDK.
//
// A Tunnel is an MCP tunnel (research-preview beta API). It is immutable after
// creation: the Anthropic API exposes no update endpoint, so changes to any
// forProvider field are not reconciled. See docs/overlays/tunnel.md.
type TunnelParameters struct {
	// DisplayName is an optional human-readable name for the tunnel (1-255
	// characters). Immutable after creation.
	// +optional
	// +kubebuilder:validation:MaxLength=255
	DisplayName *string `json:"displayName,omitempty"`
}

// TunnelObservation holds the observed state of an Anthropic Tunnel as returned
// by the API. These fields are read-only. The connector token is intentionally
// omitted — it is a credential, published as a connection detail rather than
// stored in status where it would be visible to anyone who can kubectl get.
type TunnelObservation struct {
	// ID is the Anthropic-assigned identifier (a tnl_... tagged ID). Also stored
	// in the external-name annotation, which the reconciler uses as the primary
	// key.
	// +optional
	ID *string `json:"id,omitempty"`

	// DisplayName is the observed human-readable name of the tunnel.
	// +optional
	DisplayName *string `json:"displayName,omitempty"`

	// Domain is the Anthropic-assigned hostname for the tunnel. MCP server URLs
	// whose host is a subdomain of this value are routed through the tunnel.
	// Globally unique and never reused, even after the tunnel is archived.
	// +optional
	Domain *string `json:"domain,omitempty"`

	// CreatedAt is the RFC 3339 timestamp when the tunnel was created.
	// +optional
	CreatedAt *string `json:"createdAt,omitempty"`
}

// TunnelSpec defines the desired state of Tunnel.
type TunnelSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`

	// ForProvider holds the configuration the provider reconciles against the
	// Anthropic API on every loop.
	ForProvider TunnelParameters `json:"forProvider"`
}

// TunnelStatus defines the observed state of Tunnel.
type TunnelStatus struct {
	xpv2.ManagedResourceStatus `json:",inline"`

	// AtProvider holds the observed state as returned by the Anthropic API.
	// +optional
	AtProvider TunnelObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="DOMAIN",type="string",JSONPath=".status.atProvider.domain"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,anthropic},shortName=tnl
//
// Tunnel is a managed resource representing an Anthropic MCP tunnel
// (research-preview beta API). A Tunnel is immutable after creation; its
// connector token is published as a connection detail.
type Tunnel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TunnelSpec   `json:"spec"`
	Status TunnelStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
//
// TunnelList contains a list of Tunnel.
type TunnelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Tunnel `json:"items"`
}

// TunnelKind and TunnelGroupVersionKind are used by controller setup.
var (
	TunnelKind             = "Tunnel"
	TunnelGroupVersionKind = GroupVersion.WithKind(TunnelKind)
)

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &Tunnel{}, &TunnelList{})
		return nil
	})
}

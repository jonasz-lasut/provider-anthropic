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

// InviteParameters defines the desired state of an Anthropic organization
// Invite. Invites are create-only: the Anthropic API has no update endpoint,
// so every field is immutable after creation.
type InviteParameters struct {
	// Required: Email of the user to invite. Sending the invite emails this
	// address. Immutable after creation.
	// +optional
	Email *string `json:"email,omitempty"`

	// Required: Role the invited user receives. Console and API organizations
	// accept user, developer, billing, and claude_code_user; Claude Enterprise
	// organizations accept user and managed. admin cannot be assigned through
	// the API. Immutable after creation.
	// +optional
	// +kubebuilder:validation:Enum=user;developer;billing;claude_code_user;managed
	Role *string `json:"role,omitempty"`

	// RBACGroupIDs are assigned to the user when the invite is accepted. Only
	// accepted by Claude Enterprise organizations with RBAC groups.
	// +optional
	RBACGroupIDs []string `json:"rbacGroupIds,omitempty"`
}

// InviteObservation holds the observed state of an Anthropic Invite as
// returned by the API. These fields are read-only.
type InviteObservation struct {
	// ID is the Anthropic-assigned invite identifier (invite_...). Also
	// stored in the external-name annotation, which the reconciler uses as
	// the primary key.
	// +optional
	ID *string `json:"id,omitempty"`

	// Email is the invited address.
	// +optional
	Email *string `json:"email,omitempty"`

	// Role is the role the invite grants.
	// +optional
	Role *string `json:"role,omitempty"`

	// Status is pending, accepted, expired, or deleted.
	// +optional
	Status *string `json:"status,omitempty"`

	// RBACGroupIDs are the groups assigned on acceptance.
	// +optional
	RBACGroupIDs []string `json:"rbacGroupIds,omitempty"`

	// InvitedAt is the RFC 3339 timestamp when the invite was sent.
	// +optional
	InvitedAt *string `json:"invitedAt,omitempty"`

	// ExpiresAt is the RFC 3339 timestamp when the invite expires.
	// +optional
	ExpiresAt *string `json:"expiresAt,omitempty"`

	// AcceptedAt is the RFC 3339 timestamp when the invite was accepted, if
	// it was.
	// +optional
	AcceptedAt *string `json:"acceptedAt,omitempty"`
}

// InviteSpec defines the desired state of Invite.
type InviteSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`

	// ForProvider holds the configuration the provider reconciles against the
	// Anthropic API on every loop.
	ForProvider InviteParameters `json:"forProvider"`
}

// InviteStatus defines the observed state of Invite.
type InviteStatus struct {
	xpv2.ManagedResourceStatus `json:",inline"`

	// AtProvider holds the observed state as returned by the Anthropic API.
	// +optional
	AtProvider InviteObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.atProvider.status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,anthropic},shortName=orginvite
//
// Invite is a managed resource representing an invitation of a user into an
// Anthropic organization (Admin API). Creating it emails the invitee;
// deleting it revokes a pending invite. Requires a ProviderConfig backed by
// an Admin API key.
type Invite struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InviteSpec   `json:"spec"`
	Status InviteStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
//
// InviteList contains a list of Invite.
type InviteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Invite `json:"items"`
}

// InviteKind and InviteGroupVersionKind are used by controller setup.
var (
	InviteKind             = "Invite"
	InviteGroupVersionKind = GroupVersion.WithKind(InviteKind)
)

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &Invite{}, &InviteList{})
		return nil
	})
}

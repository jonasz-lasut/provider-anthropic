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

// WorkspaceMemberParameters defines the desired state of an Anthropic
// WorkspaceMember. These fields map to
// BetaOrganizationWorkspaceMemberAddParams /
// BetaOrganizationWorkspaceMemberUpdateParams from the Anthropic SDK.
type WorkspaceMemberParameters struct {
	// WorkspaceID is the ID of the workspace the user is a member of.
	// Populate directly or via WorkspaceIDRef / WorkspaceIDSelector.
	// Immutable after creation.
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

	// Required: UserID is the ID of the organization user (user_...) to add
	// to the workspace. Immutable after creation. Organization admins and
	// billing members are implicit members of every workspace and cannot be
	// added or removed.
	// +optional
	UserID *string `json:"userId,omitempty"`

	// Required: WorkspaceRole is the user's role in the workspace. The
	// workspace_billing role cannot be assigned; it is inherited from the
	// organization billing role.
	// +optional
	// +kubebuilder:validation:Enum=workspace_admin;workspace_developer;workspace_restricted_developer;workspace_user
	WorkspaceRole *string `json:"workspaceRole,omitempty"`
}

// WorkspaceMemberObservation holds the observed state of an Anthropic
// WorkspaceMember as returned by the API. Memberships have no identifier of
// their own: the pair (workspaceId, userId) addresses them, and the
// external-name annotation stores the user ID.
type WorkspaceMemberObservation struct {
	// WorkspaceID is the observed workspace ID.
	// +optional
	WorkspaceID *string `json:"workspaceId,omitempty"`

	// UserID is the observed user ID.
	// +optional
	UserID *string `json:"userId,omitempty"`

	// WorkspaceRole is the observed role, which may be workspace_billing for
	// members who inherit it from the organization billing role.
	// +optional
	WorkspaceRole *string `json:"workspaceRole,omitempty"`
}

// WorkspaceMemberSpec defines the desired state of WorkspaceMember.
type WorkspaceMemberSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`

	// ForProvider holds the configuration the provider reconciles against the
	// Anthropic API on every loop.
	ForProvider WorkspaceMemberParameters `json:"forProvider"`
}

// WorkspaceMemberStatus defines the observed state of WorkspaceMember.
type WorkspaceMemberStatus struct {
	xpv2.ManagedResourceStatus `json:",inline"`

	// AtProvider holds the observed state as returned by the Anthropic API.
	// +optional
	AtProvider WorkspaceMemberObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,anthropic},shortName=wrkspcmember
//
// WorkspaceMember is a managed resource representing a user's membership in
// an Anthropic organization workspace (Admin API). Requires a ProviderConfig
// backed by an Admin API key.
type WorkspaceMember struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkspaceMemberSpec   `json:"spec"`
	Status WorkspaceMemberStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
//
// WorkspaceMemberList contains a list of WorkspaceMember.
type WorkspaceMemberList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkspaceMember `json:"items"`
}

// WorkspaceMemberKind and WorkspaceMemberGroupVersionKind are used by
// controller setup.
var (
	WorkspaceMemberKind             = "WorkspaceMember"
	WorkspaceMemberGroupVersionKind = GroupVersion.WithKind(WorkspaceMemberKind)
)

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &WorkspaceMember{}, &WorkspaceMemberList{})
		return nil
	})
}

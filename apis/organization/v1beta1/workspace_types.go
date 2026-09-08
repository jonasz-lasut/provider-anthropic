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

// WorkspaceDataResidency configures where a workspace stores data and which
// inference geos it may use. The same type serves ForProvider and AtProvider
// so the structured drift diff compares the nested object key by key.
type WorkspaceDataResidency struct {
	// WorkspaceGeo is the geographic region for workspace data storage.
	// Immutable after creation; the API defaults it to "us".
	// +optional
	// +kubebuilder:validation:Enum=us
	WorkspaceGeo *string `json:"workspaceGeo,omitempty"`

	// AllowedInferenceGeos lists the inference geos requests may use. The
	// single entry "unrestricted" allows every geo, which is the API default.
	// +optional
	// +kubebuilder:validation:items:Enum=unrestricted;global;us
	AllowedInferenceGeos []string `json:"allowedInferenceGeos,omitempty"`

	// DefaultInferenceGeo applies when a request omits the parameter. It must
	// be listed in AllowedInferenceGeos unless that is "unrestricted"; the API
	// defaults it to "global".
	// +optional
	// +kubebuilder:validation:Enum=global;us
	DefaultInferenceGeo *string `json:"defaultInferenceGeo,omitempty"`
}

// WorkspaceParameters defines the desired state of an Anthropic Workspace.
// These fields map to BetaOrganizationWorkspaceNewParams /
// BetaOrganizationWorkspaceUpdateParams from the Anthropic SDK.
type WorkspaceParameters struct {
	// Required: Name of the workspace.
	// +optional
	Name *string `json:"name,omitempty"`

	// DisplayColor is the hex color code representing the workspace in the
	// Anthropic Console, e.g. "#6C5BB9".
	// +optional
	DisplayColor *string `json:"displayColor,omitempty"`

	// ExternalKeyID is the ID of the customer-managed encryption key (CMEK)
	// configuration for this workspace. Requires CMEK to be enabled for the
	// organization and is write-once: once a key is attached it cannot be
	// detached or replaced.
	// +optional
	ExternalKeyID *string `json:"externalKeyId,omitempty"`

	// Tags are user-defined string key-value pairs. Keys may not begin with
	// "anthropic". The provider merges its canonical crossplane-* identifier
	// tags into this map unless started with --skip-default-metadata.
	// +optional
	Tags map[string]string `json:"tags,omitempty"`

	// DataResidency configures data storage and inference geos. When omitted
	// the API defaults to workspaceGeo "us", allowedInferenceGeos
	// ["unrestricted"], and defaultInferenceGeo "global".
	// +optional
	DataResidency *WorkspaceDataResidency `json:"dataResidency,omitempty"`
}

// WorkspaceObservation holds the observed state of an Anthropic Workspace
// as returned by the API. These fields are read-only.
type WorkspaceObservation struct {
	// ID is the Anthropic-assigned workspace identifier (wrkspc_...). Also
	// stored in the external-name annotation, which the reconciler uses as
	// the primary key.
	// +optional
	ID *string `json:"id,omitempty"`

	// Name is the observed workspace name.
	// +optional
	Name *string `json:"name,omitempty"`

	// DisplayColor is the observed Console color.
	// +optional
	DisplayColor *string `json:"displayColor,omitempty"`

	// ExternalKeyID is the attached CMEK key configuration, if any.
	// +optional
	ExternalKeyID *string `json:"externalKeyId,omitempty"`

	// Tags is the observed tag map.
	// +optional
	Tags map[string]string `json:"tags,omitempty"`

	// DataResidency is the observed data residency configuration.
	// +optional
	DataResidency *WorkspaceDataResidency `json:"dataResidency,omitempty"`

	// CompartmentID is the ID of the compartment that holds the workspace.
	// +optional
	CompartmentID *string `json:"compartmentId,omitempty"`

	// CreatedAt is the RFC 3339 timestamp when the workspace was created.
	// +optional
	CreatedAt *string `json:"createdAt,omitempty"`

	// ArchivedAt is set when the workspace has been archived.
	// +optional
	ArchivedAt *string `json:"archivedAt,omitempty"`
}

// WorkspaceSpec defines the desired state of Workspace.
type WorkspaceSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`

	// ForProvider holds the configuration the provider reconciles against the
	// Anthropic API on every loop.
	ForProvider WorkspaceParameters `json:"forProvider"`
}

// WorkspaceStatus defines the observed state of Workspace.
type WorkspaceStatus struct {
	xpv2.ManagedResourceStatus `json:",inline"`

	// AtProvider holds the observed state as returned by the Anthropic API.
	// +optional
	AtProvider WorkspaceObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,anthropic},shortName=wrkspc
//
// Workspace is a managed resource representing an Anthropic organization
// workspace (Admin API). Deleting it archives the workspace, which the
// Anthropic API cannot undo. Requires a ProviderConfig backed by an Admin API
// key.
type Workspace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkspaceSpec   `json:"spec"`
	Status WorkspaceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
//
// WorkspaceList contains a list of Workspace.
type WorkspaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workspace `json:"items"`
}

// WorkspaceKind and WorkspaceGroupVersionKind are used by controller setup.
var (
	WorkspaceKind             = "Workspace"
	WorkspaceGroupVersionKind = GroupVersion.WithKind(WorkspaceKind)
)

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &Workspace{}, &WorkspaceList{})
		return nil
	})
}

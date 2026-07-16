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

// EnvironmentNetworkingConfig configures the network policy for a cloud environment.
type EnvironmentNetworkingConfig struct {
	// Required: Type is the network policy: "unrestricted" allows full outbound access,
	// "limited" restricts outbound traffic to the hosts listed in AllowedHosts.
	// +optional
	// +kubebuilder:validation:Enum=unrestricted;limited
	Type *string `json:"type,omitempty"`

	// AllowMCPServers permits outbound access to MCP server endpoints configured
	// on the agent, beyond those listed in AllowedHosts. Limited networking only.
	// +optional
	AllowMCPServers *bool `json:"allowMcpServers,omitempty"`

	// AllowPackageManagers permits outbound access to public package registries
	// (PyPI, npm, etc.) beyond those listed in AllowedHosts. Limited networking only.
	// +optional
	AllowPackageManagers *bool `json:"allowPackageManagers,omitempty"`

	// AllowedHosts specifies additional domains the container can reach.
	// Limited networking only.
	// +optional
	// +listType=set
	AllowedHosts []string `json:"allowedHosts,omitempty"`
}

// EnvironmentPackages specifies packages available in the environment.
type EnvironmentPackages struct {
	// Apt lists Ubuntu/Debian packages to install.
	// +optional
	// +listType=set
	Apt []string `json:"apt,omitempty"`

	// Cargo lists Rust packages to install.
	// +optional
	// +listType=set
	Cargo []string `json:"cargo,omitempty"`

	// Gem lists Ruby packages to install.
	// +optional
	// +listType=set
	Gem []string `json:"gem,omitempty"`

	// Go lists Go packages to install.
	// +optional
	// +listType=set
	Go []string `json:"go,omitempty"`

	// Npm lists Node.js packages to install.
	// +optional
	// +listType=set
	Npm []string `json:"npm,omitempty"`

	// Pip lists Python packages to install.
	// +optional
	// +listType=set
	Pip []string `json:"pip,omitempty"`
}

// EnvironmentCloudConfig configures a cloud environment.
type EnvironmentCloudConfig struct {
	// Networking configures the network policy. Omit to preserve the existing value on update.
	// +optional
	Networking *EnvironmentNetworkingConfig `json:"networking,omitempty"`

	// Packages specifies packages available in the environment.
	// Omit to preserve the existing value on update.
	// +optional
	Packages *EnvironmentPackages `json:"packages,omitempty"`
}

// EnvironmentParameters defines the desired state of an Anthropic Environment.
// These fields map to BetaEnvironmentNewParams / BetaEnvironmentUpdateParams
// from the Anthropic SDK.
type EnvironmentParameters struct {
	// Required: Name is the human-readable name for the environment.
	// +optional
	Name *string `json:"name,omitempty"`

	// Description is an optional description of the environment.
	// +optional
	Description *string `json:"description,omitempty"`

	// Config is the cloud environment configuration.
	// +optional
	Config *EnvironmentCloudConfig `json:"config,omitempty"`

	// Metadata is arbitrary key-value data attached to the environment.
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`

	// Scope controls the visibility of the environment. "organization" makes it
	// visible to all accounts; "account" restricts it to the owning account.
	// Only applicable for self-hosted environments.
	// +optional
	// +kubebuilder:validation:Enum=organization;account
	Scope *string `json:"scope,omitempty"`

	// AnthropicDeletionPolicy controls whether Crossplane calls Archive or
	// Delete on the Anthropic API when the resource is deleted in Kubernetes.
	// +optional
	// +kubebuilder:validation:Enum=Archive;Delete
	// +kubebuilder:default=Archive
	AnthropicDeletionPolicy *string `json:"anthropicDeletionPolicy,omitempty"`
}

// EnvironmentObservation holds the observed state of an Anthropic Environment
// as returned by the API. These fields are read-only.
type EnvironmentObservation struct {
	// ID is the Anthropic-assigned environment identifier. Also stored in the
	// external-name annotation, which the reconciler uses as the primary key.
	// +optional
	ID *string `json:"id,omitempty"`

	// Name is the observed human-readable environment name.
	// +optional
	Name *string `json:"name,omitempty"`

	// Description is the observed user-provided description.
	// +optional
	Description *string `json:"description,omitempty"`

	// Config is the observed cloud environment configuration.
	// +optional
	Config *EnvironmentCloudConfig `json:"config,omitempty"`

	// Metadata is the observed key-value metadata map.
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`

	// CreatedAt is the RFC 3339 timestamp when the environment was created.
	// +optional
	CreatedAt *string `json:"createdAt,omitempty"`

	// UpdatedAt is the RFC 3339 timestamp of the last modification.
	// +optional
	UpdatedAt *string `json:"updatedAt,omitempty"`

	// ArchivedAt is set when the environment has been archived.
	// +optional
	ArchivedAt *string `json:"archivedAt,omitempty"`

	// Scope is the observed visibility scope of the environment.
	// +optional
	Scope *string `json:"scope,omitempty"`
}

// EnvironmentSpec defines the desired state of Environment.
type EnvironmentSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`

	// ForProvider holds the configuration the provider reconciles against the
	// Anthropic API on every loop.
	ForProvider EnvironmentParameters `json:"forProvider"`
}

// EnvironmentStatus defines the observed state of Environment.
type EnvironmentStatus struct {
	xpv2.ManagedResourceStatus `json:",inline"`

	// AtProvider holds the observed state as returned by the Anthropic API.
	// +optional
	AtProvider EnvironmentObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,anthropic},shortName=env
//
// Environment is a managed resource representing an Anthropic cloud environment
// (beta API).
type Environment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EnvironmentSpec   `json:"spec"`
	Status EnvironmentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
//
// EnvironmentList contains a list of Environment.
type EnvironmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Environment `json:"items"`
}

// EnvironmentKind and EnvironmentGroupVersionKind are used by controller setup.
var (
	EnvironmentKind             = "Environment"
	EnvironmentGroupVersionKind = GroupVersion.WithKind(EnvironmentKind)
)

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &Environment{}, &EnvironmentList{})
		return nil
	})
}

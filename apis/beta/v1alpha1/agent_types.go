/*
Copyright 2025 The provider-anthropic-platform Authors.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	v2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// AgentParameters defines the desired state of an Anthropic Managed Agent.
// These fields map directly to the BetaAgentNewParams / BetaAgentUpdateParams
// from the Anthropic SDK and are always reconciled against the external API.
type AgentParameters struct {
	// Required: Model is the model identifier for the agent, e.g. "claude-opus-4-7".
	// +optional
	Model *string `json:"model,omitempty"`

	// Required: Name is the human-readable name for the agent (1–256 characters).
	// +optional
	// +kubebuilder:validation:MaxLength=256
	Name *string `json:"name,omitempty"`

	// Description of what the agent does. Up to 2048 characters.
	// +optional
	// +kubebuilder:validation:MaxLength=2048
	Description *string `json:"description,omitempty"`

	// SystemSecretRef references a Secret in the MR's namespace holding the
	// system prompt at the given key. Up to 100,000 characters; the API
	// rejects larger payloads. Omit to send no system prompt.
	// +optional
	SystemSecretRef *xpv1.LocalSecretKeySelector `json:"systemSecretRef,omitempty"`

	// MCPServers this agent connects to. Maximum 20. Names must be unique.
	// +optional
	// +kubebuilder:validation:MaxItems=20
	MCPServers []MCPServerConfig `json:"mcpServers,omitempty"`

	// Metadata is arbitrary key-value data attached to the agent.
	// Maximum 16 pairs; keys up to 64 chars, values up to 512 chars.
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`

	// Skills available to the agent. Maximum 20.
	// +optional
	// +kubebuilder:validation:MaxItems=20
	Skills []AgentSkillConfig `json:"skills,omitempty"`

	// Tools available to the agent. Maximum 128 tools across all toolsets.
	// +optional
	// +kubebuilder:validation:MaxItems=128
	Tools []AgentToolConfig `json:"tools,omitempty"`
}

// MCPServerConfig configures a single MCP server for an agent.
type MCPServerConfig struct {
	// Required: Name is a unique identifier for this MCP server within the agent.
	// +optional
	Name *string `json:"name,omitempty"`

	// Required: URL is the endpoint of the MCP server.
	// +optional
	URL *string `json:"url,omitempty"`
}

// AgentSkillConfig configures a single skill for an agent.
type AgentSkillConfig struct {
	// Required: Type is either "anthropic" or "custom".
	// +optional
	// +kubebuilder:validation:Enum=anthropic;custom
	Type *string `json:"type,omitempty"`

	// Required: SkillID is the identifier of the skill to attach.
	// +optional
	SkillID *string `json:"skillId,omitempty"`
}

// AgentToolConfig configures a single tool (toolset) for an agent.
type AgentToolConfig struct {
	// Required: Type identifies the toolset kind; currently only
	// "agent_toolset_20260401" is supported for the standard Anthropic toolset.
	// +optional
	Type *string `json:"type,omitempty"`

	// Name is the toolset name as recognised by the Anthropic API.
	// +optional
	Name *string `json:"name,omitempty"`
}

// AgentModelObservation is the observed model configuration returned by the API.
type AgentModelObservation struct {
	// ID is the model identifier, e.g. "claude-opus-4-7".
	// +optional
	ID *string `json:"id,omitempty"`
}

// AgentObservation holds the observed state of an Anthropic Managed Agent as
// returned by the API.  These fields are read-only.
type AgentObservation struct {
	// ID is the Anthropic-assigned agent identifier. Also stored in the
	// external-name annotation, which the reconciler uses as the primary key.
	// +optional
	ID *string `json:"id,omitempty"`

	// Name is the observed human-readable name.
	// +optional
	Name *string `json:"name,omitempty"`

	// Description is the observed description.
	// +optional
	Description *string `json:"description,omitempty"`

	// Model is the observed model configuration.
	// +optional
	Model *AgentModelObservation `json:"model,omitempty"`

	// System is the observed system prompt (populated from resp.System when the
	// agent has a system prompt configured).
	// +optional
	System *string `json:"system,omitempty"`

	// MCPServers is the observed list of MCP server configurations.
	// +optional
	MCPServers []MCPServerConfig `json:"mcpServers,omitempty"`

	// Skills is the observed list of skill configurations.
	// +optional
	Skills []AgentSkillConfig `json:"skills,omitempty"`

	// Tools is the observed list of tool configurations.
	// +optional
	Tools []AgentToolConfig `json:"tools,omitempty"`

	// Metadata is the observed key-value metadata map.
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`

	// CreatedAt is the RFC 3339 timestamp when the agent was created.
	// +optional
	CreatedAt *string `json:"createdAt,omitempty"`

	// UpdatedAt is the RFC 3339 timestamp of the last modification.
	// +optional
	UpdatedAt *string `json:"updatedAt,omitempty"`

	// ArchivedAt is set when the agent has been archived.
	// +optional
	ArchivedAt *string `json:"archivedAt,omitempty"`

	// Version is incremented each time the agent is modified and is required
	// for optimistic-concurrency updates.
	// +optional
	Version *int64 `json:"version,omitempty"`
}

// AgentSpec defines the desired state of Agent.
type AgentSpec struct {
	v2.ManagedResourceSpec `json:",inline"`

	// ForProvider holds the configuration the provider reconciles against the
	// Anthropic API on every loop.
	ForProvider AgentParameters `json:"forProvider"`
}

// AgentStatus defines the observed state of Agent.
type AgentStatus struct {
	xpv1.ResourceStatus `json:",inline"`

	// AtProvider holds the observed state as returned by the Anthropic API.
	// +optional
	AtProvider AgentObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,anthropic},shortName=agent
//
// Agent is a managed resource representing an Anthropic Managed Agent
// (beta API).
type Agent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AgentSpec   `json:"spec"`
	Status AgentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
//
// AgentList contains a list of Agent.
type AgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Agent `json:"items"`
}

// AgentKind and AgentGroupVersionKind are used by controller setup.
var (
	AgentKind             = "Agent"
	AgentGroupVersionKind = GroupVersion.WithKind(AgentKind)
)

func init() {
	SchemeBuilder.Register(&Agent{}, &AgentList{})
}

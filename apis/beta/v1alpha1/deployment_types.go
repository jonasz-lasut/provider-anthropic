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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	v2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// DeploymentBlockSource is the source for an image or document content block
// in a Deployment initial event. The Type field selects the variant; only set
// the fields relevant to that variant.
type DeploymentBlockSource struct {
	// Required: Type identifies the source variant: "base64", "url", "file",
	// or "text" (text is document-only, media type text/plain).
	// +optional
	// +kubebuilder:validation:Enum=base64;url;file;text
	Type *string `json:"type,omitempty"`

	// Data is the inline content (base64-encoded for base64 sources, plain text
	// for text sources).
	// +optional
	Data *string `json:"data,omitempty"`

	// MediaType is the MIME type of the inline data (base64 and text sources),
	// e.g. "image/png", "application/pdf", "text/plain".
	// +optional
	MediaType *string `json:"mediaType,omitempty"`

	// URL is the location to fetch the content from (url source).
	// +optional
	URL *string `json:"url,omitempty"`

	// FileID is the ID of a previously uploaded file (file source).
	// +optional
	FileID *string `json:"fileId,omitempty"`
}

// DeploymentContentBlock is a single content block in a user.message (text,
// image, or document) or system.message (text only) initial event.
type DeploymentContentBlock struct {
	// Required: Type identifies the block variant: "text", "image", or
	// "document". system.message events accept only "text".
	// +optional
	// +kubebuilder:validation:Enum=text;image;document
	Type *string `json:"type,omitempty"`

	// Text is the block content (text block).
	// +optional
	Text *string `json:"text,omitempty"`

	// Source is the image or document source (image and document blocks).
	// +optional
	Source *DeploymentBlockSource `json:"source,omitempty"`

	// Context is additional context about the document for the model
	// (document block).
	// +optional
	Context *string `json:"context,omitempty"`

	// Title is the title of the document (document block).
	// +optional
	Title *string `json:"title,omitempty"`
}

// DeploymentRubric grades the quality of an outcome for a user.define_outcome
// initial event. The Type field selects the variant.
type DeploymentRubric struct {
	// Required: Type identifies the rubric variant: "text" or "file".
	// +optional
	// +kubebuilder:validation:Enum=text;file
	Type *string `json:"type,omitempty"`

	// Content is the rubric content as plain text or markdown (text variant).
	// Maximum 262144 characters.
	// +optional
	Content *string `json:"content,omitempty"`

	// FileID is the ID of the rubric file (file variant).
	// +optional
	FileID *string `json:"fileId,omitempty"`
}

// DeploymentInitialEvent is one event sent to each session immediately after
// creation. The Type field selects the variant; only set the fields relevant
// to that variant.
type DeploymentInitialEvent struct {
	// Required: Type identifies the event variant: "user.message",
	// "user.define_outcome", or "system.message".
	// +optional
	// +kubebuilder:validation:Enum=user.message;user.define_outcome;system.message
	Type *string `json:"type,omitempty"`

	// Content holds the message content blocks for "user.message" (text, image,
	// document) and "system.message" (text only) events.
	// +optional
	Content []DeploymentContentBlock `json:"content,omitempty"`

	// Description is the task specification for a "user.define_outcome" event.
	// +optional
	Description *string `json:"description,omitempty"`

	// MaxIterations is the number of eval/revision cycles before giving up for a
	// "user.define_outcome" event. Default 3, maximum 20.
	// +optional
	MaxIterations *int64 `json:"maxIterations,omitempty"`

	// Rubric grades the quality of the outcome for a "user.define_outcome" event.
	// +optional
	Rubric *DeploymentRubric `json:"rubric,omitempty"`
}

// DeploymentSchedule is the cron schedule on which the deployment creates
// sessions.
type DeploymentSchedule struct {
	// Required: Expression is a 5-field POSIX cron expression (minute hour
	// day-of-month month day-of-week), e.g. "0 9 * * 1-5". Extended cron syntax
	// and predefined shortcuts are not supported.
	// +optional
	Expression *string `json:"expression,omitempty"`

	// Required: Timezone is an IANA timezone identifier (e.g.
	// "America/Los_Angeles", "UTC").
	// +optional
	Timezone *string `json:"timezone,omitempty"`
}

// DeploymentParameters defines the desired state of an Anthropic Deployment.
// These fields map to BetaDeploymentNewParams / BetaDeploymentUpdateParams
// from the Anthropic SDK.
type DeploymentParameters struct {
	// Required: Name is the human-readable name for the deployment.
	// +optional
	Name *string `json:"name,omitempty"`

	// Description of what the deployment does.
	// +optional
	Description *string `json:"description,omitempty"`

	// AgentID is the ID of the Agent to deploy. Accepts an agent ID string
	// (re-pins the latest version) or use AgentVersion to pin a specific version.
	// Populate directly or via AgentIDRef / AgentIDSelector.
	// +crossplane:generate:reference:type=github.com/jonasz-lasut/provider-anthropic/apis/beta/v1alpha1.Agent
	// +crossplane:generate:reference:extractor=github.com/jonasz-lasut/provider-anthropic/internal/extractors.ComputedFieldExtractor("id")
	// +optional
	AgentID *string `json:"agentId,omitempty"`

	// Reference to an Agent to populate agentId.
	// +kubebuilder:validation:Optional
	AgentIDRef *xpv1.NamespacedReference `json:"agentIdRef,omitempty"`

	// Selector for an Agent to populate agentId.
	// +kubebuilder:validation:Optional
	AgentIDSelector *xpv1.NamespacedSelector `json:"agentIdSelector,omitempty"`

	// AgentVersion pins a specific agent version. When nil, the latest version
	// of the referenced agent is used.
	// +optional
	AgentVersion *int64 `json:"agentVersion,omitempty"`

	// EnvironmentID is the ID of the Environment defining the container
	// configuration for sessions created from this deployment.
	// Populate directly or via EnvironmentIDRef / EnvironmentIDSelector.
	// +crossplane:generate:reference:type=github.com/jonasz-lasut/provider-anthropic/apis/beta/v1alpha1.Environment
	// +crossplane:generate:reference:extractor=github.com/jonasz-lasut/provider-anthropic/internal/extractors.ComputedFieldExtractor("id")
	// +optional
	EnvironmentID *string `json:"environmentId,omitempty"`

	// Reference to an Environment to populate environmentId.
	// +kubebuilder:validation:Optional
	EnvironmentIDRef *xpv1.NamespacedReference `json:"environmentIdRef,omitempty"`

	// Selector for an Environment to populate environmentId.
	// +kubebuilder:validation:Optional
	EnvironmentIDSelector *xpv1.NamespacedSelector `json:"environmentIdSelector,omitempty"`

	// Required: InitialEvents are sent to each session immediately after
	// creation. At least 1, maximum 50.
	// +optional
	// +kubebuilder:validation:MaxItems=50
	InitialEvents []DeploymentInitialEvent `json:"initialEvents,omitempty"`

	// Resources (e.g. repositories, files, memory stores) to mount into each
	// session's container. Maximum 500.
	// +optional
	// +kubebuilder:validation:MaxItems=500
	Resources []SessionResource `json:"resources,omitempty"`

	// Schedule is the cron schedule on which the deployment creates sessions.
	// +optional
	Schedule *DeploymentSchedule `json:"schedule,omitempty"`

	// VaultIDs lists vault IDs for stored credentials the agent can use during
	// sessions created from this deployment. Maximum 50.
	// Populate directly or via VaultIDsRefs / VaultIDsSelector.
	// +crossplane:generate:reference:type=github.com/jonasz-lasut/provider-anthropic/apis/beta/v1alpha1.Vault
	// +crossplane:generate:reference:extractor=github.com/jonasz-lasut/provider-anthropic/internal/extractors.ComputedFieldExtractor("id")
	// +optional
	// +listType=set
	VaultIDs []string `json:"vaultIds,omitempty"`

	// References to Vaults used to populate vaultIds.
	// +kubebuilder:validation:Optional
	VaultIDsRefs []xpv1.NamespacedReference `json:"vaultIdsRefs,omitempty"`

	// Selector for Vaults used to populate vaultIds.
	// +kubebuilder:validation:Optional
	VaultIDsSelector *xpv1.NamespacedSelector `json:"vaultIdsSelector,omitempty"`

	// Metadata is arbitrary key-value data attached to the deployment.
	// Maximum 16 pairs; keys up to 64 chars, values up to 512 chars.
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`

	// Paused controls the deployment lifecycle. Set to true to pause the
	// schedule (the reconciler calls Pause); set to false (or omit) to keep it
	// active (the reconciler calls Unpause when needed).
	// +optional
	Paused *bool `json:"paused,omitempty"`
}

// DeploymentScheduleObservation is the observed cron schedule with computed
// runtime timestamps.
type DeploymentScheduleObservation struct {
	// Expression is the observed cron expression.
	// +optional
	Expression *string `json:"expression,omitempty"`

	// Timezone is the observed IANA timezone identifier.
	// +optional
	Timezone *string `json:"timezone,omitempty"`

	// LastRunAt is the RFC 3339 timestamp of the last schedule run.
	// +optional
	LastRunAt *string `json:"lastRunAt,omitempty"`

	// UpcomingRunsAt lists RFC 3339 timestamps of upcoming cron occurrences.
	// +optional
	UpcomingRunsAt []string `json:"upcomingRunsAt,omitempty"`
}

// DeploymentObservation holds the observed state of an Anthropic Deployment as
// returned by the API. These fields are read-only.
//
// initialEvents and resources are intentionally not mirrored here: they are
// deep, API-normalized structures whose round-trip would risk false drift in
// the structured diff (see isUpToDate). They are applied on create and on any
// update triggered by drift in another field.
type DeploymentObservation struct {
	// ID is the Anthropic-assigned identifier. Also stored in the external-name
	// annotation, which the reconciler uses as the primary key.
	// +optional
	ID *string `json:"id,omitempty"`

	// Name is the observed human-readable deployment name.
	// +optional
	Name *string `json:"name,omitempty"`

	// Description is the observed description.
	// +optional
	Description *string `json:"description,omitempty"`

	// AgentID is the resolved agent ID bound to this deployment.
	// +optional
	AgentID *string `json:"agentId,omitempty"`

	// AgentVersion is the resolved agent version bound to this deployment.
	// +optional
	AgentVersion *int64 `json:"agentVersion,omitempty"`

	// EnvironmentID is the observed environment ID.
	// +optional
	EnvironmentID *string `json:"environmentId,omitempty"`

	// VaultIDs is the observed list of vault IDs.
	// +optional
	VaultIDs []string `json:"vaultIds,omitempty"`

	// Metadata is the observed key-value metadata map.
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`

	// Schedule is the observed cron schedule with computed runtime timestamps.
	// +optional
	Schedule *DeploymentScheduleObservation `json:"schedule,omitempty"`

	// Status is the lifecycle status: "active" or "paused".
	// +optional
	Status *string `json:"status,omitempty"`

	// Paused mirrors the desired pause state derived from Status, so drift in
	// spec.forProvider.paused is detected.
	// +optional
	Paused *bool `json:"paused,omitempty"`

	// CreatedAt is the RFC 3339 timestamp when the deployment was created.
	// +optional
	CreatedAt *string `json:"createdAt,omitempty"`

	// UpdatedAt is the RFC 3339 timestamp of the last modification.
	// +optional
	UpdatedAt *string `json:"updatedAt,omitempty"`

	// ArchivedAt is set when the deployment has been archived.
	// +optional
	ArchivedAt *string `json:"archivedAt,omitempty"`
}

// DeploymentSpec defines the desired state of Deployment.
type DeploymentSpec struct {
	v2.ManagedResourceSpec `json:",inline"`

	// ForProvider holds the configuration the provider reconciles against the
	// Anthropic API on every loop.
	ForProvider DeploymentParameters `json:"forProvider"`
}

// DeploymentStatus defines the observed state of Deployment.
type DeploymentStatus struct {
	xpv1.ResourceStatus `json:",inline"`

	// AtProvider holds the observed state as returned by the Anthropic API.
	// +optional
	AtProvider DeploymentObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,anthropic},shortName=dpl
//
// Deployment is a managed resource representing an Anthropic Deployment (beta
// API): a scheduled agent run that materializes sessions on a cron schedule.
type Deployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeploymentSpec   `json:"spec"`
	Status DeploymentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
//
// DeploymentList contains a list of Deployment.
type DeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Deployment `json:"items"`
}

// DeploymentKind and DeploymentGroupVersionKind are used by controller setup.
var (
	DeploymentKind             = "Deployment"
	DeploymentGroupVersionKind = GroupVersion.WithKind(DeploymentKind)
)

func init() {
	SchemeBuilder.Register(&Deployment{}, &DeploymentList{})
}

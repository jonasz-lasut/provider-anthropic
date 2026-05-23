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

// SkillParameters defines the desired state of an Anthropic Skill.
type SkillParameters struct {
	// DisplayTitle is the human-readable label for the skill.
	// Immutable after creation — the Anthropic API provides no endpoint to
	// update BetaSkill. Changes require delete and recreate.
	// +optional
	DisplayTitle *string `json:"displayTitle,omitempty"`

	// Required: FilesSecretRef references a Secret in the MR's namespace where
	// each key is a flat filename (e.g. "SKILL.md", "helpers.py") and each
	// value is the file content in bytes. The Secret must contain a "SKILL.md"
	// key at minimum. The reconciler automatically places all files under a
	// top-level directory named after the Skill resource (e.g. the Skill named
	// "my-skill" uploads files as "my-skill/SKILL.md"). Content changes trigger
	// a new SkillVersion.
	FilesSecretRef xpv1.LocalSecretReference `json:"filesSecretRef"`
}

// SkillObservation holds the observed state of an Anthropic Skill.
type SkillObservation struct {
	// ID is the Anthropic-assigned skill identifier (skl_...). Also stored in
	// the external-name annotation, which the reconciler uses as the primary key.
	// +optional
	ID *string `json:"id,omitempty"`

	// DisplayTitle is the observed human-readable label.
	// +optional
	DisplayTitle *string `json:"displayTitle,omitempty"`

	// Source is either "custom" (user-created) or "anthropic".
	// +optional
	Source *string `json:"source,omitempty"`

	// CreatedAt is the ISO 8601 timestamp when the skill was created.
	// +optional
	CreatedAt *string `json:"createdAt,omitempty"`

	// UpdatedAt is the ISO 8601 timestamp of the last modification.
	// +optional
	UpdatedAt *string `json:"updatedAt,omitempty"`

	// LatestVersion is the version string of the most recent SkillVersion
	// (a Unix epoch timestamp, e.g. "1759178010641129").
	// +optional
	LatestVersion *string `json:"latestVersion,omitempty"`

	// LatestVersionID is the Anthropic-assigned ID of the latest SkillVersion.
	// +optional
	LatestVersionID *string `json:"latestVersionId,omitempty"`

	// LatestVersionName is extracted from SKILL.md by the Anthropic API.
	// +optional
	LatestVersionName *string `json:"latestVersionName,omitempty"`

	// LatestVersionDescription is extracted from SKILL.md by the Anthropic API.
	// +optional
	LatestVersionDescription *string `json:"latestVersionDescription,omitempty"`

	// LatestVersionDirectory is the top-level directory name extracted from the
	// uploaded file paths (e.g. "myskill").
	// +optional
	LatestVersionDirectory *string `json:"latestVersionDirectory,omitempty"`

	// LatestVersionCreatedAt is the ISO 8601 timestamp when the latest version
	// was created.
	// +optional
	LatestVersionCreatedAt *string `json:"latestVersionCreatedAt,omitempty"`

	// FilesSha256 is the full 64-char hex SHA-256 of the canonical encoding of
	// Secret.Data at the time of the last successful upload. Used for drift
	// detection; raw file content is never stored here.
	// +optional
	FilesSha256 *string `json:"filesSha256,omitempty"`
}

// SkillSpec defines the desired state of Skill.
type SkillSpec struct {
	v2.ManagedResourceSpec `json:",inline"`

	// ForProvider holds the configuration the provider reconciles against the
	// Anthropic API on every loop.
	ForProvider SkillParameters `json:"forProvider"`
}

// SkillStatus defines the observed state of Skill.
type SkillStatus struct {
	xpv1.ResourceStatus `json:",inline"`

	// AtProvider holds the observed state as returned by the Anthropic API.
	// +optional
	AtProvider SkillObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,anthropic},shortName=skill
//
// Skill is a managed resource representing an Anthropic Skill (beta API).
// It manages both the BetaSkill container and its BetaSkillVersion content.
type Skill struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SkillSpec   `json:"spec"`
	Status SkillStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
//
// SkillList contains a list of Skill.
type SkillList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Skill `json:"items"`
}

// SkillKind and SkillGroupVersionKind are used by controller setup.
var (
	SkillKind             = "Skill"
	SkillGroupVersionKind = GroupVersion.WithKind(SkillKind)
)

func init() {
	SchemeBuilder.Register(&Skill{}, &SkillList{})
}

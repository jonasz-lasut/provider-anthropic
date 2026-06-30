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
	anthropic "github.com/anthropics/anthropic-sdk-go"
)

// ToAnthropicNew converts ForProvider to BetaSkillNewParams.
// Files are NOT included — they are assembled by the reconciler from the
// staged filesystem and appended to params.Files before calling the SDK.
func (r *Skill) ToAnthropicNew() anthropic.BetaSkillNewParams {
	params := anthropic.BetaSkillNewParams{}
	if r.Spec.ForProvider.DisplayTitle != nil {
		params.DisplayTitle = anthropic.String(*r.Spec.ForProvider.DisplayTitle)
	}
	return params
}

// ToAnthropicNewVersion returns BetaSkillVersionNewParams.
// Files are NOT included — assembled by the reconciler before calling the SDK.
func (r *Skill) ToAnthropicNewVersion() anthropic.BetaSkillVersionNewParams {
	return anthropic.BetaSkillVersionNewParams{}
}

// FromAnthropicSkillObservation populates AtProvider from a BetaSkillGetResponse.
func (r *Skill) FromAnthropicSkillObservation(resp anthropic.BetaSkillGetResponse) {
	r.Status.AtProvider.ID = &resp.ID
	r.Status.AtProvider.DisplayTitle = &resp.DisplayTitle
	r.Status.AtProvider.Source = &resp.Source
	r.Status.AtProvider.CreatedAt = &resp.CreatedAt
	r.Status.AtProvider.UpdatedAt = &resp.UpdatedAt
	r.Status.AtProvider.LatestVersion = &resp.LatestVersion
}

// FromAnthropicVersionObservation populates AtProvider version fields from a
// BetaSkillVersionGetResponse.
func (r *Skill) FromAnthropicVersionObservation(resp anthropic.BetaSkillVersionGetResponse) {
	r.Status.AtProvider.LatestVersionID = &resp.ID
	r.Status.AtProvider.LatestVersionName = &resp.Name
	r.Status.AtProvider.LatestVersionDescription = &resp.Description
	r.Status.AtProvider.LatestVersionDirectory = &resp.Directory
	r.Status.AtProvider.LatestVersionCreatedAt = &resp.CreatedAt
}

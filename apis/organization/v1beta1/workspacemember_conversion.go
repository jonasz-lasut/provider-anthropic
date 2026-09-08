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

// ToAnthropicNew converts ForProvider to the params of Members.Add. The
// workspace ID is a positional argument of that call and is not part of the
// params.
func (r *WorkspaceMember) ToAnthropicNew() anthropic.BetaOrganizationWorkspaceMemberAddParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaOrganizationWorkspaceMemberAddParams{}
	if p.UserID != nil {
		params.UserID = *p.UserID
	}
	if p.WorkspaceRole != nil {
		params.WorkspaceRole = anthropic.BetaNoBillingWorkspaceRole(*p.WorkspaceRole)
	}
	return params
}

// ToAnthropicUpdate converts ForProvider to the params of Members.Update,
// which carries the workspace ID inside the params while the user ID is
// positional.
func (r *WorkspaceMember) ToAnthropicUpdate() anthropic.BetaOrganizationWorkspaceMemberUpdateParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaOrganizationWorkspaceMemberUpdateParams{}
	if p.WorkspaceID != nil {
		params.WorkspaceID = *p.WorkspaceID
	}
	if p.WorkspaceRole != nil {
		params.WorkspaceRole = anthropic.BetaWorkspaceRole(*p.WorkspaceRole)
	}
	return params
}

// FromAnthropicObservation populates AtProvider from a BetaWorkspaceMember.
func (r *WorkspaceMember) FromAnthropicObservation(resp anthropic.BetaWorkspaceMember) {
	r.Status.AtProvider.WorkspaceID = &resp.WorkspaceID
	r.Status.AtProvider.UserID = &resp.UserID
	role := string(resp.WorkspaceRole)
	r.Status.AtProvider.WorkspaceRole = &role
}

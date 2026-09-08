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
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
)

// ToAnthropicNew converts ForProvider to BetaOrganizationInviteNewParams.
func (r *Invite) ToAnthropicNew() anthropic.BetaOrganizationInviteNewParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaOrganizationInviteNewParams{}
	if p.Email != nil {
		params.Email = *p.Email
	}
	if p.Role != nil {
		params.Role = anthropic.BetaOrganizationInviteNewParamsRole(*p.Role)
	}
	if p.RBACGroupIDs != nil {
		params.RBACGroupIDs = p.RBACGroupIDs
	}
	return params
}

// FromAnthropicObservation populates AtProvider from a BetaOrganizationInvite.
func (r *Invite) FromAnthropicObservation(resp anthropic.BetaOrganizationInvite) {
	r.Status.AtProvider.ID = &resp.ID
	r.Status.AtProvider.Email = &resp.Email
	role := string(resp.Role)
	r.Status.AtProvider.Role = &role
	status := string(resp.Status)
	r.Status.AtProvider.Status = &status
	r.Status.AtProvider.RBACGroupIDs = nil
	if len(resp.RBACGroupIDs) > 0 {
		r.Status.AtProvider.RBACGroupIDs = resp.RBACGroupIDs
	}
	invitedAt := resp.InvitedAt.Format(time.RFC3339)
	r.Status.AtProvider.InvitedAt = &invitedAt
	expiresAt := resp.ExpiresAt.Format(time.RFC3339)
	r.Status.AtProvider.ExpiresAt = &expiresAt
	r.Status.AtProvider.AcceptedAt = nil
	if !resp.AcceptedAt.IsZero() {
		acceptedAt := resp.AcceptedAt.Format(time.RFC3339)
		r.Status.AtProvider.AcceptedAt = &acceptedAt
	}
}

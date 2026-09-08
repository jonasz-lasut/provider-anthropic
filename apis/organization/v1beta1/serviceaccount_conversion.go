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

// ToAnthropicNew converts ForProvider to BetaOrganizationServiceAccountNewParams.
func (r *ServiceAccount) ToAnthropicNew() anthropic.BetaOrganizationServiceAccountNewParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaOrganizationServiceAccountNewParams{}
	if p.Name != nil {
		params.Name = *p.Name
	}
	if p.Description != nil {
		params.Description = anthropic.String(*p.Description)
	}
	if p.OrganizationRole != nil {
		params.OrganizationRole = anthropic.BetaOrganizationServiceAccountNewParamsOrganizationRole(*p.OrganizationRole)
	}
	return params
}

// ToAnthropicUpdate converts ForProvider to
// BetaOrganizationServiceAccountUpdateParams. Name is immutable and not sent.
func (r *ServiceAccount) ToAnthropicUpdate() anthropic.BetaOrganizationServiceAccountUpdateParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaOrganizationServiceAccountUpdateParams{}
	if p.Description != nil {
		params.Description = anthropic.String(*p.Description)
	}
	if p.OrganizationRole != nil {
		params.OrganizationRole = anthropic.BetaOrganizationServiceAccountUpdateParamsOrganizationRole(*p.OrganizationRole)
	}
	return params
}

// FromAnthropicObservation populates AtProvider from a BetaServiceAccount.
// ArchivedAt is intentionally omitted: the reconciler treats an archived
// service account as absent.
func (r *ServiceAccount) FromAnthropicObservation(resp anthropic.BetaServiceAccount) {
	r.Status.AtProvider.ID = &resp.ID
	r.Status.AtProvider.Name = &resp.Name
	r.Status.AtProvider.Description = &resp.Description
	role := string(resp.OrganizationRole)
	r.Status.AtProvider.OrganizationRole = &role
	createdAt := resp.CreatedAt.Format(time.RFC3339)
	r.Status.AtProvider.CreatedAt = &createdAt
	updatedAt := resp.UpdatedAt.Format(time.RFC3339)
	r.Status.AtProvider.UpdatedAt = &updatedAt
	r.Status.AtProvider.CreatedByActorID = optionalString(resp.CreatedByActorID)
	r.Status.AtProvider.UpdatedByActorID = optionalString(resp.UpdatedByActorID)
}

// optionalString returns a pointer to s, or nil when s is empty, so optional
// API fields stay absent from status instead of showing as "".
func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// optionalTime formats t as RFC 3339, or returns nil for the zero time.
func optionalTime(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

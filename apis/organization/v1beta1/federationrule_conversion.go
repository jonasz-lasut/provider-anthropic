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

// ToAnthropicNew converts ForProvider to BetaOrganizationFederationRuleNewParams.
func (r *FederationRule) ToAnthropicNew() anthropic.BetaOrganizationFederationRuleNewParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaOrganizationFederationRuleNewParams{}
	if p.IssuerID != nil {
		params.IssuerID = *p.IssuerID
	}
	if p.Name != nil {
		params.Name = *p.Name
	}
	if p.OAuthScope != nil {
		params.OAuthScope = *p.OAuthScope
	}
	if p.ServiceAccountID != nil {
		params.Target = anthropic.BetaServiceAccountTargetParam{ServiceAccountID: *p.ServiceAccountID}
	}
	if p.Match != nil {
		params.Match = matchParam(p.Match)
	}
	if p.Description != nil {
		params.Description = anthropic.String(*p.Description)
	}
	if p.WorkspaceID != nil {
		params.WorkspaceID = anthropic.String(*p.WorkspaceID)
	}
	if p.AppliesToAllWorkspaces != nil {
		params.AppliesToAllWorkspaces = anthropic.Bool(*p.AppliesToAllWorkspaces)
	}
	if p.TokenLifetimeSeconds != nil {
		params.TokenLifetimeSeconds = anthropic.Int(*p.TokenLifetimeSeconds)
	}
	return params
}

// ToAnthropicUpdate converts ForProvider to BetaOrganizationFederationRuleUpdateParams.
func (r *FederationRule) ToAnthropicUpdate() anthropic.BetaOrganizationFederationRuleUpdateParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaOrganizationFederationRuleUpdateParams{}
	if p.Name != nil {
		params.Name = anthropic.String(*p.Name)
	}
	if p.Description != nil {
		params.Description = anthropic.String(*p.Description)
	}
	if p.OAuthScope != nil {
		params.OAuthScope = anthropic.String(*p.OAuthScope)
	}
	if p.ServiceAccountID != nil {
		params.Target = anthropic.BetaServiceAccountTargetParam{ServiceAccountID: *p.ServiceAccountID}
	}
	if p.Match != nil {
		params.Match = matchParam(p.Match)
	}
	if p.WorkspaceID != nil {
		params.WorkspaceID = anthropic.String(*p.WorkspaceID)
	}
	if p.AppliesToAllWorkspaces != nil {
		params.AppliesToAllWorkspaces = anthropic.Bool(*p.AppliesToAllWorkspaces)
	}
	if p.TokenLifetimeSeconds != nil {
		params.TokenLifetimeSeconds = anthropic.Int(*p.TokenLifetimeSeconds)
	}
	return params
}

func matchParam(m *FederationRuleMatch) anthropic.BetaFederationRuleMatchParam {
	out := anthropic.BetaFederationRuleMatchParam{}
	if m.SubjectPrefix != nil {
		out.SubjectPrefix = anthropic.String(*m.SubjectPrefix)
	}
	if m.Audience != nil {
		out.Audience = anthropic.String(*m.Audience)
	}
	if m.Condition != nil {
		out.Condition = anthropic.String(*m.Condition)
	}
	if m.Claims != nil {
		out.Claims = m.Claims
	}
	return out
}

// FromAnthropicObservation populates AtProvider from a BetaFederationRule.
// ArchivedAt is intentionally omitted: the reconciler treats an archived
// rule as absent.
func (r *FederationRule) FromAnthropicObservation(resp anthropic.BetaFederationRule) {
	r.Status.AtProvider.ID = &resp.ID
	r.Status.AtProvider.Name = &resp.Name
	r.Status.AtProvider.Description = &resp.Description
	r.Status.AtProvider.IssuerID = &resp.IssuerID
	r.Status.AtProvider.IssuerName = optionalString(resp.IssuerName)
	r.Status.AtProvider.ServiceAccountID = optionalString(resp.Target.ServiceAccountID)
	r.Status.AtProvider.ServiceAccountName = optionalString(resp.Target.ServiceAccountName)
	r.Status.AtProvider.OAuthScope = &resp.OAuthScope
	match := &FederationRuleMatch{
		SubjectPrefix: optionalString(resp.Match.SubjectPrefix),
		Audience:      optionalString(resp.Match.Audience),
		Condition:     optionalString(resp.Match.Condition),
	}
	if len(resp.Match.Claims) > 0 {
		match.Claims = resp.Match.Claims
	}
	r.Status.AtProvider.Match = match
	r.Status.AtProvider.WorkspaceID = optionalString(resp.WorkspaceID)
	r.Status.AtProvider.WorkspaceIDs = nil
	if len(resp.WorkspaceIDs) > 0 {
		r.Status.AtProvider.WorkspaceIDs = resp.WorkspaceIDs
	}
	all := resp.AppliesToAllWorkspaces
	r.Status.AtProvider.AppliesToAllWorkspaces = &all
	lifetime := resp.TokenLifetimeSeconds
	r.Status.AtProvider.TokenLifetimeSeconds = &lifetime
	r.Status.AtProvider.CreatedAt = optionalTime(resp.CreatedAt)
	r.Status.AtProvider.UpdatedAt = optionalTime(resp.UpdatedAt)
	r.Status.AtProvider.CreatedByActorID = optionalString(resp.CreatedByActorID)
	r.Status.AtProvider.UpdatedByActorID = optionalString(resp.UpdatedByActorID)
}

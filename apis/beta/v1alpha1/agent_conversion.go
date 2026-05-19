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
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
)

// AgentConversionContext carries reconcile-time values needed for Agent SDK
// param construction. Pass nil when no secret resolution is needed.
type AgentConversionContext struct {
	System string // resolved from SystemSecretRef; empty if not set
}

func (r *Agent) ToAnthropicNew(ctx *AgentConversionContext) anthropic.BetaAgentNewParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaAgentNewParams{}
	if p.Name != nil {
		params.Name = *p.Name
	}
	if p.Model != nil {
		params.Model = anthropic.BetaManagedAgentsModelConfigParams{ID: *p.Model}
	}
	if p.Description != nil {
		params.Description = anthropic.String(*p.Description)
	}
	if ctx != nil && ctx.System != "" {
		params.System = anthropic.String(ctx.System)
	}
	if p.Metadata != nil {
		params.Metadata = p.Metadata
	}
	for _, s := range p.MCPServers {
		srv := anthropic.BetaManagedAgentsURLMCPServerParams{
			Type: anthropic.BetaManagedAgentsURLMCPServerParamsTypeURL,
		}
		if s.Name != nil {
			srv.Name = *s.Name
		}
		if s.URL != nil {
			srv.URL = *s.URL
		}
		params.MCPServers = append(params.MCPServers, srv)
	}
	for _, sk := range p.Skills {
		params.Skills = append(params.Skills, agentSkillToParam(sk))
	}
	for _, t := range p.Tools {
		params.Tools = append(params.Tools, agentToolToNewParam(t))
	}
	return params
}

func (r *Agent) ToAnthropicUpdate(ctx *AgentConversionContext) anthropic.BetaAgentUpdateParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaAgentUpdateParams{}
	if r.Status.AtProvider.Version != nil {
		params.Version = *r.Status.AtProvider.Version
	}
	if p.Name != nil {
		params.Name = anthropic.String(*p.Name)
	}
	if p.Model != nil {
		params.Model = anthropic.BetaManagedAgentsModelConfigParams{ID: *p.Model}
	}
	if p.Description != nil {
		params.Description = anthropic.String(*p.Description)
	}
	if ctx != nil && ctx.System != "" {
		params.System = anthropic.String(ctx.System)
	}
	if p.Metadata != nil {
		params.Metadata = p.Metadata
	}
	for _, s := range p.MCPServers {
		srv := anthropic.BetaManagedAgentsURLMCPServerParams{
			Type: anthropic.BetaManagedAgentsURLMCPServerParamsTypeURL,
		}
		if s.Name != nil {
			srv.Name = *s.Name
		}
		if s.URL != nil {
			srv.URL = *s.URL
		}
		params.MCPServers = append(params.MCPServers, srv)
	}
	for _, sk := range p.Skills {
		params.Skills = append(params.Skills, agentSkillToParam(sk))
	}
	for _, t := range p.Tools {
		params.Tools = append(params.Tools, agentToolToUpdateParam(t))
	}
	return params
}

func (r *Agent) FromAnthropicObservation(resp anthropic.BetaManagedAgentsAgent) {
	r.Status.AtProvider.ID = &resp.ID
	r.Status.AtProvider.Name = &resp.Name
	r.Status.AtProvider.Description = &resp.Description
	r.Status.AtProvider.System = &resp.System
	r.Status.AtProvider.Version = &resp.Version
	r.Status.AtProvider.Metadata = resp.Metadata

	modelID := string(resp.Model.ID)
	r.Status.AtProvider.Model = &AgentModelObservation{ID: &modelID}

	r.Status.AtProvider.MCPServers = nil
	for _, s := range resp.MCPServers {
		name, url := s.Name, s.URL
		r.Status.AtProvider.MCPServers = append(r.Status.AtProvider.MCPServers, MCPServerConfig{
			Name: &name,
			URL:  &url,
		})
	}

	r.Status.AtProvider.Skills = nil
	for _, sk := range resp.Skills {
		skillID, skillType := sk.SkillID, sk.Type
		r.Status.AtProvider.Skills = append(r.Status.AtProvider.Skills, AgentSkillConfig{
			SkillID: &skillID,
			Type:    &skillType,
		})
	}

	r.Status.AtProvider.Tools = nil
	for _, t := range resp.Tools {
		toolType := t.Type
		r.Status.AtProvider.Tools = append(r.Status.AtProvider.Tools, AgentToolConfig{
			Type: &toolType,
		})
	}

	createdAt := resp.CreatedAt.Format(time.RFC3339)
	r.Status.AtProvider.CreatedAt = &createdAt
	updatedAt := resp.UpdatedAt.Format(time.RFC3339)
	r.Status.AtProvider.UpdatedAt = &updatedAt
	// ArchivedAt intentionally omitted
}

func agentSkillToParam(s AgentSkillConfig) anthropic.BetaManagedAgentsSkillParamsUnion {
	skillID := ""
	if s.SkillID != nil {
		skillID = *s.SkillID
	}
	skillType := ""
	if s.Type != nil {
		skillType = *s.Type
	}
	switch skillType {
	case "anthropic":
		return anthropic.BetaManagedAgentsSkillParamsUnion{
			OfAnthropic: &anthropic.BetaManagedAgentsAnthropicSkillParams{
				SkillID: skillID,
				Type:    anthropic.BetaManagedAgentsAnthropicSkillParamsTypeAnthropic,
			},
		}
	case "custom":
		fallthrough
	default:
		return anthropic.BetaManagedAgentsSkillParamsUnion{
			OfCustom: &anthropic.BetaManagedAgentsCustomSkillParams{
				SkillID: skillID,
				Type:    anthropic.BetaManagedAgentsCustomSkillParamsTypeCustom,
			},
		}
	}
}

func agentToolToNewParam(_ AgentToolConfig) anthropic.BetaAgentNewParamsToolUnion {
	return anthropic.BetaAgentNewParamsToolUnion{
		OfAgentToolset20260401: &anthropic.BetaManagedAgentsAgentToolset20260401Params{
			Type: anthropic.BetaManagedAgentsAgentToolset20260401ParamsTypeAgentToolset20260401,
		},
	}
}

func agentToolToUpdateParam(_ AgentToolConfig) anthropic.BetaAgentUpdateParamsToolUnion {
	return anthropic.BetaAgentUpdateParamsToolUnion{
		OfAgentToolset20260401: &anthropic.BetaManagedAgentsAgentToolset20260401Params{
			Type: anthropic.BetaManagedAgentsAgentToolset20260401ParamsTypeAgentToolset20260401,
		},
	}
}

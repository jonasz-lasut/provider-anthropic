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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"k8s.io/apimachinery/pkg/runtime"
)

// AgentConversionContext carries reconcile-time values needed for Agent SDK
// param construction. Pass nil when no secret resolution is needed.
type AgentConversionContext struct {
	System string // resolved from SystemSecretRef; empty if not set
}

// ToConnectionDetails publishes all non-empty resolved secret values as
// Crossplane connection details so consumers can access them via
// spec.writeConnectionSecretToRef.
func (cctx *AgentConversionContext) ToConnectionDetails() managed.ConnectionDetails {
	cd := managed.ConnectionDetails{}
	if cctx.System != "" {
		cd["system"] = []byte(cctx.System)
	}
	return cd
}

func (r *Agent) ToAnthropicNew(ctx *AgentConversionContext) anthropic.BetaAgentNewParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaAgentNewParams{}
	if p.Name != nil {
		params.Name = *p.Name
	}
	if p.Model != nil || p.ModelEffort != nil || p.ModelInferenceGeo != nil {
		params.Model = agentModelConfigToParam(p.Model, p.ModelEffort, p.ModelInferenceGeo)
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
	if p.Multiagent != nil {
		params.Multiagent = agentMultiagentToParam(p.Multiagent)
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

func agentModelConfigToParam(model, effort, inferenceGeo *string) anthropic.BetaManagedAgentsModelConfigParams {
	mc := anthropic.BetaManagedAgentsModelConfigParams{}
	if model != nil {
		mc.ID = *model
	}
	if effort != nil {
		mc.Effort = anthropic.BetaManagedAgentsModelConfigParamsEffortUnion{
			OfBetaManagedAgentsModelConfigsEffortBetaManagedAgentsEffortLevel: anthropic.String(*effort),
		}
	}
	if inferenceGeo != nil {
		mc.InferenceGeo = anthropic.String(*inferenceGeo)
	}
	return mc
}

func (r *Agent) ToAnthropicUpdate(ctx *AgentConversionContext) anthropic.BetaAgentUpdateParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaAgentUpdateParams{}
	if r.Status.AtProvider.Version != nil {
		params.Version = anthropic.Int(*r.Status.AtProvider.Version)
	}
	if p.Name != nil {
		params.Name = anthropic.String(*p.Name)
	}
	if p.Model != nil || p.ModelEffort != nil || p.ModelInferenceGeo != nil {
		params.Model = agentModelConfigToParam(p.Model, p.ModelEffort, p.ModelInferenceGeo)
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
	if p.Multiagent != nil {
		params.Multiagent = agentMultiagentToParam(p.Multiagent)
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
	r.Status.AtProvider.Version = &resp.Version
	// System intentionally omitted — stored only as SHA-256 for drift detection.
	if resp.System != "" {
		sum := sha256.Sum256([]byte(resp.System))
		s := hex.EncodeToString(sum[:])
		r.Status.AtProvider.SystemSha256 = &s
	} else {
		r.Status.AtProvider.SystemSha256 = nil
	}
	r.Status.AtProvider.Metadata = resp.Metadata

	modelID := string(resp.Model.ID)
	r.Status.AtProvider.Model = &AgentModelObservation{ID: &modelID}
	if resp.Model.Effort.Type != "" {
		effort := resp.Model.Effort.Type
		r.Status.AtProvider.ModelEffort = &effort
	} else {
		r.Status.AtProvider.ModelEffort = nil
	}
	if resp.Model.InferenceGeo != "" {
		geo := resp.Model.InferenceGeo
		r.Status.AtProvider.ModelInferenceGeo = &geo
	} else {
		r.Status.AtProvider.ModelInferenceGeo = nil
	}

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
		r.Status.AtProvider.Skills = append(r.Status.AtProvider.Skills, AgentSkillObservation{
			SkillID: &skillID,
			Type:    &skillType,
		})
	}

	r.Status.AtProvider.Tools = nil
	for _, t := range resp.Tools {
		r.Status.AtProvider.Tools = append(r.Status.AtProvider.Tools, agentToolFromObservation(t))
	}

	r.Status.AtProvider.Multiagent = agentMultiagentFromObservation(resp.Multiagent, resp.ID)

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

func agentMultiagentToParam(m *AgentMultiagentConfig) anthropic.BetaManagedAgentsMultiagentParams {
	params := anthropic.BetaManagedAgentsMultiagentParams{
		Type: anthropic.BetaManagedAgentsMultiagentParamsTypeCoordinator,
	}
	for _, e := range m.Agents {
		params.Agents = append(params.Agents, agentRosterEntryToParam(e))
	}
	return params
}

func agentRosterEntryToParam(e AgentRosterEntry) anthropic.BetaManagedAgentsMultiagentRosterEntryParamsUnion {
	entryType := ""
	if e.Type != nil {
		entryType = *e.Type
	}
	switch entryType {
	case "self":
		return anthropic.BetaManagedAgentsMultiagentRosterEntryParamsUnion{
			OfBetaManagedAgentsMultiagentSelfs: &anthropic.BetaManagedAgentsMultiagentSelfParams{
				Type: anthropic.BetaManagedAgentsMultiagentSelfParamsTypeSelf,
			},
		}
	case "advisor":
		model := ""
		if e.Model != nil {
			model = *e.Model
		}
		return anthropic.BetaManagedAgentsMultiagentRosterEntryParamsUnion{
			OfBetaManagedAgentsAdvisors: &anthropic.BetaManagedAgentsAdvisorParams{
				Model: model,
				Type:  anthropic.BetaManagedAgentsAdvisorParamsTypeAdvisor,
			},
		}
	case "agent":
		fallthrough
	default:
		ref := &anthropic.BetaManagedAgentsAgentParams{
			Type: anthropic.BetaManagedAgentsAgentParamsTypeAgent,
		}
		if e.ID != nil {
			ref.ID = *e.ID
		}
		if e.Version != nil {
			ref.Version = anthropic.Int(*e.Version)
		}
		return anthropic.BetaManagedAgentsMultiagentRosterEntryParamsUnion{
			OfBetaManagedAgentsAgents: ref,
		}
	}
}

// agentMultiagentFromObservation maps the resolved roster into status. The API
// resolves "self" entries to concrete agent references, so an entry whose ID
// matches the agent's own ID is reported back as type "self" — otherwise a
// desired {type: self} entry would drift forever. Its resolved id and version
// are kept as extra keys, which the subset comparison ignores.
func agentMultiagentFromObservation(m anthropic.BetaManagedAgentsMultiagent, ownID string) *AgentMultiagentObservation {
	if m.Type == "" {
		return nil
	}
	obs := &AgentMultiagentObservation{}
	for _, e := range m.Agents {
		entryType := e.Type
		entry := AgentRosterEntryObservation{Type: &entryType}
		switch e.Type {
		case "advisor":
			model := e.Model
			entry.Model = &model
		default:
			id, version := e.ID, e.Version
			if id == ownID {
				self := "self"
				entry.Type = &self
			}
			entry.ID = &id
			entry.Version = &version
		}
		obs.Agents = append(obs.Agents, entry)
	}
	return obs
}

func agentToolToNewParam(t AgentToolConfig) anthropic.BetaAgentNewParamsToolUnion {
	switch agentToolType(t) {
	case "mcp_toolset":
		return anthropic.BetaAgentNewParamsToolUnion{OfMCPToolset: mcpToolsetParam(t)}
	case "custom":
		return anthropic.BetaAgentNewParamsToolUnion{OfCustom: customToolParam(t)}
	default:
		return anthropic.BetaAgentNewParamsToolUnion{OfAgentToolset20260401: agentToolsetParam(t)}
	}
}

func agentToolToUpdateParam(t AgentToolConfig) anthropic.BetaAgentUpdateParamsToolUnion {
	switch agentToolType(t) {
	case "mcp_toolset":
		return anthropic.BetaAgentUpdateParamsToolUnion{OfMCPToolset: mcpToolsetParam(t)}
	case "custom":
		return anthropic.BetaAgentUpdateParamsToolUnion{OfCustom: customToolParam(t)}
	default:
		return anthropic.BetaAgentUpdateParamsToolUnion{OfAgentToolset20260401: agentToolsetParam(t)}
	}
}

func agentToolType(t AgentToolConfig) string {
	if t.Type != nil {
		return *t.Type
	}
	return ""
}

func agentToolsetParam(t AgentToolConfig) *anthropic.BetaManagedAgentsAgentToolset20260401Params {
	p := &anthropic.BetaManagedAgentsAgentToolset20260401Params{
		Type: anthropic.BetaManagedAgentsAgentToolset20260401ParamsTypeAgentToolset20260401,
	}
	if d := t.DefaultConfig; d != nil {
		p.DefaultConfig = anthropic.BetaManagedAgentsAgentToolsetDefaultConfigParams{
			Enabled:          optionalBool(d.Enabled),
			PermissionPolicy: toolPermissionPolicy(d.PermissionPolicy),
		}
	}
	for _, c := range t.Configs {
		p.Configs = append(p.Configs, agentToolOverrideParam(c))
	}
	return p
}

func mcpToolsetParam(t AgentToolConfig) *anthropic.BetaManagedAgentsMCPToolsetParams {
	p := &anthropic.BetaManagedAgentsMCPToolsetParams{
		Type: anthropic.BetaManagedAgentsMCPToolsetParamsTypeMCPToolset,
	}
	if t.MCPServerName != nil {
		p.MCPServerName = *t.MCPServerName
	}
	if d := t.DefaultConfig; d != nil {
		p.DefaultConfig = anthropic.BetaManagedAgentsMCPToolsetDefaultConfigParams{
			Enabled:          optionalBool(d.Enabled),
			PermissionPolicy: anthropic.BetaManagedAgentsMCPToolsetDefaultConfigParamsPermissionPolicyUnion(toolPermissionPolicy(d.PermissionPolicy)),
		}
	}
	for _, c := range t.Configs {
		mc := anthropic.BetaManagedAgentsMCPToolConfigParams{
			Enabled:          optionalBool(c.Enabled),
			PermissionPolicy: anthropic.BetaManagedAgentsMCPToolConfigParamsPermissionPolicyUnion(toolPermissionPolicy(c.PermissionPolicy)),
		}
		if c.Name != nil {
			mc.Name = *c.Name
		}
		p.Configs = append(p.Configs, mc)
	}
	return p
}

func customToolParam(t AgentToolConfig) *anthropic.BetaManagedAgentsCustomToolParams {
	p := &anthropic.BetaManagedAgentsCustomToolParams{
		Type: anthropic.BetaManagedAgentsCustomToolParamsTypeCustom,
	}
	if t.Name != nil {
		p.Name = *t.Name
	}
	if t.Description != nil {
		p.Description = *t.Description
	}
	if t.InputSchema != nil {
		if len(t.InputSchema.Properties.Raw) > 0 {
			var props map[string]any
			_ = json.Unmarshal(t.InputSchema.Properties.Raw, &props)
			p.InputSchema.Properties = props
		}
		p.InputSchema.Required = t.InputSchema.Required
	}
	return p
}

// agentToolOverrideParam builds the per-tool config variant selected by the
// tool name. An unknown name yields a zero-value union, which the API rejects.
func agentToolOverrideParam(c AgentToolOverride) anthropic.BetaManagedAgentsAgentToolConfigParamsUnion {
	enabled := optionalBool(c.Enabled)
	policy := toolPermissionPolicy(c.PermissionPolicy)
	name := ""
	if c.Name != nil {
		name = *c.Name
	}
	switch name {
	case "bash":
		return anthropic.BetaManagedAgentsAgentToolConfigParamsUnion{OfBash: &anthropic.BetaManagedAgentsBashToolConfigParams{
			Enabled:          enabled,
			PermissionPolicy: anthropic.BetaManagedAgentsBashToolConfigParamsPermissionPolicyUnion(policy),
			Type:             anthropic.BetaManagedAgentsBashToolConfigParamsTypeBash,
		}}
	case "edit":
		return anthropic.BetaManagedAgentsAgentToolConfigParamsUnion{OfEdit: &anthropic.BetaManagedAgentsEditToolConfigParams{
			Enabled:          enabled,
			PermissionPolicy: anthropic.BetaManagedAgentsEditToolConfigParamsPermissionPolicyUnion(policy),
			Type:             anthropic.BetaManagedAgentsEditToolConfigParamsTypeEdit,
		}}
	case "read":
		return anthropic.BetaManagedAgentsAgentToolConfigParamsUnion{OfRead: &anthropic.BetaManagedAgentsReadToolConfigParams{
			Enabled:          enabled,
			PermissionPolicy: anthropic.BetaManagedAgentsReadToolConfigParamsPermissionPolicyUnion(policy),
			Type:             anthropic.BetaManagedAgentsReadToolConfigParamsTypeRead,
		}}
	case "write":
		return anthropic.BetaManagedAgentsAgentToolConfigParamsUnion{OfWrite: &anthropic.BetaManagedAgentsWriteToolConfigParams{
			Enabled:          enabled,
			PermissionPolicy: anthropic.BetaManagedAgentsWriteToolConfigParamsPermissionPolicyUnion(policy),
			Type:             anthropic.BetaManagedAgentsWriteToolConfigParamsTypeWrite,
		}}
	case "glob":
		return anthropic.BetaManagedAgentsAgentToolConfigParamsUnion{OfGlob: &anthropic.BetaManagedAgentsGlobToolConfigParams{
			Enabled:          enabled,
			PermissionPolicy: anthropic.BetaManagedAgentsGlobToolConfigParamsPermissionPolicyUnion(policy),
			Type:             anthropic.BetaManagedAgentsGlobToolConfigParamsTypeGlob,
		}}
	case "grep":
		return anthropic.BetaManagedAgentsAgentToolConfigParamsUnion{OfGrep: &anthropic.BetaManagedAgentsGrepToolConfigParams{
			Enabled:          enabled,
			PermissionPolicy: anthropic.BetaManagedAgentsGrepToolConfigParamsPermissionPolicyUnion(policy),
			Type:             anthropic.BetaManagedAgentsGrepToolConfigParamsTypeGrep,
		}}
	case "web_fetch":
		wf := &anthropic.BetaManagedAgentsWebFetchToolConfigParams{
			Enabled:          enabled,
			PermissionPolicy: anthropic.BetaManagedAgentsWebFetchToolConfigParamsPermissionPolicyUnion(policy),
			AllowedDomains:   c.AllowedDomains,
			BlockedDomains:   c.BlockedDomains,
			Type:             anthropic.BetaManagedAgentsWebFetchToolConfigParamsTypeWebFetch,
		}
		if c.MaxContentTokens != nil {
			wf.MaxContentTokens = anthropic.Int(*c.MaxContentTokens)
		}
		return anthropic.BetaManagedAgentsAgentToolConfigParamsUnion{OfWebFetch: wf}
	case "web_search":
		ws := &anthropic.BetaManagedAgentsWebSearchToolConfigParams{
			Enabled:          enabled,
			PermissionPolicy: anthropic.BetaManagedAgentsWebSearchToolConfigParamsPermissionPolicyUnion(policy),
			AllowedDomains:   c.AllowedDomains,
			BlockedDomains:   c.BlockedDomains,
			Type:             anthropic.BetaManagedAgentsWebSearchToolConfigParamsTypeWebSearch,
		}
		if l := c.UserLocation; l != nil {
			ws.UserLocation = anthropic.BetaManagedAgentsUserLocationParam{
				City:     optionalString(l.City),
				Country:  optionalString(l.Country),
				Region:   optionalString(l.Region),
				Timezone: optionalString(l.Timezone),
			}
		}
		return anthropic.BetaManagedAgentsAgentToolConfigParamsUnion{OfWebSearch: ws}
	}
	return anthropic.BetaManagedAgentsAgentToolConfigParamsUnion{}
}

// toolPermissionPolicy builds the permission policy union. Every toolset and
// per-tool config declares its own union type with the same fields, so callers
// convert the result to theirs.
func toolPermissionPolicy(p *string) anthropic.BetaManagedAgentsAgentToolsetDefaultConfigParamsPermissionPolicyUnion {
	var u anthropic.BetaManagedAgentsAgentToolsetDefaultConfigParamsPermissionPolicyUnion
	if p == nil {
		return u
	}
	switch *p {
	case "always_allow":
		u.OfAlwaysAllow = &anthropic.BetaManagedAgentsAlwaysAllowPolicyParam{Type: anthropic.BetaManagedAgentsAlwaysAllowPolicyTypeAlwaysAllow}
	case "always_ask":
		u.OfAlwaysAsk = &anthropic.BetaManagedAgentsAlwaysAskPolicyParam{Type: anthropic.BetaManagedAgentsAlwaysAskPolicyTypeAlwaysAsk}
	case "auto":
		auto := anthropic.NewBetaManagedAgentsAutoPolicyParam()
		u.OfAuto = &auto
	}
	return u
}

func agentToolFromObservation(t anthropic.BetaManagedAgentsAgentToolUnion) AgentToolConfig {
	toolType := t.Type
	cfg := AgentToolConfig{Type: &toolType}
	switch toolType {
	case "agent_toolset_20260401":
		ts := t.AsAgentToolset20260401()
		cfg.DefaultConfig = toolsetDefaultFromObservation(ts.DefaultConfig.Enabled, ts.DefaultConfig.PermissionPolicy.Type)
		for _, c := range ts.Configs {
			cfg.Configs = append(cfg.Configs, agentToolOverrideFromObservation(c))
		}
	case "mcp_toolset":
		mcpName := t.MCPServerName
		cfg.MCPServerName = &mcpName
		ms := t.AsMCPToolset()
		cfg.DefaultConfig = toolsetDefaultFromObservation(ms.DefaultConfig.Enabled, ms.DefaultConfig.PermissionPolicy.Type)
		for _, c := range ms.Configs {
			name, enabled, policy := c.Name, c.Enabled, c.PermissionPolicy.Type
			cfg.Configs = append(cfg.Configs, AgentToolOverride{Name: &name, Enabled: &enabled, PermissionPolicy: &policy})
		}
	case "custom":
		name, desc := t.Name, t.Description
		cfg.Name = &name
		cfg.Description = &desc
		if t.InputSchema.RawJSON() != "" {
			schema := &AgentCustomToolInputSchema{
				Required: t.InputSchema.Required,
			}
			if t.InputSchema.Properties != nil {
				raw, _ := json.Marshal(t.InputSchema.Properties)
				schema.Properties = runtime.RawExtension{Raw: raw}
			}
			cfg.InputSchema = schema
		}
	}
	return cfg
}

// toolsetDefaultFromObservation returns nil when the response carries no
// default config, which leaves an omitted spec.defaultConfig drift-free.
func toolsetDefaultFromObservation(enabled bool, policy string) *AgentToolsetDefaultConfig {
	if policy == "" {
		return nil
	}
	return &AgentToolsetDefaultConfig{Enabled: &enabled, PermissionPolicy: &policy}
}

func agentToolOverrideFromObservation(c anthropic.BetaManagedAgentsAgentToolConfigUnion) AgentToolOverride {
	name, enabled, policy := c.Name, c.Enabled, c.PermissionPolicy.Type
	o := AgentToolOverride{Name: &name, Enabled: &enabled, PermissionPolicy: &policy}
	if len(c.AllowedDomains) > 0 {
		o.AllowedDomains = c.AllowedDomains
	}
	if len(c.BlockedDomains) > 0 {
		o.BlockedDomains = c.BlockedDomains
	}
	if c.MaxContentTokens != 0 {
		maxTokens := c.MaxContentTokens
		o.MaxContentTokens = &maxTokens
	}
	if l := c.UserLocation; l.City != "" || l.Country != "" || l.Region != "" || l.Timezone != "" {
		o.UserLocation = &AgentToolUserLocation{
			City:     nonEmpty(l.City),
			Country:  nonEmpty(l.Country),
			Region:   nonEmpty(l.Region),
			Timezone: nonEmpty(l.Timezone),
		}
	}
	return o
}

func optionalBool(b *bool) param.Opt[bool] {
	if b == nil {
		return param.Opt[bool]{}
	}
	return anthropic.Bool(*b)
}

func optionalString(s *string) param.Opt[string] {
	if s == nil {
		return param.Opt[string]{}
	}
	return anthropic.String(*s)
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

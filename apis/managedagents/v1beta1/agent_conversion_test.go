package v1beta1_test

import (
	"testing"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/jonasz-lasut/provider-anthropic/apis/managedagents/v1beta1"
)

func ptr[T any](v T) *T { return &v }

func TestAgentToAnthropicNew(t *testing.T) {
	r := &Agent{
		Spec: AgentSpec{ForProvider: AgentParameters{
			Name:        ptr("my-agent"),
			Model:       ptr("claude-opus-4-7"),
			Description: ptr("desc"),
			Metadata:    map[string]string{"k": "v"},
			MCPServers:  []MCPServerConfig{{Name: ptr("srv"), URL: ptr("https://mcp.example")}},
			Skills:      []AgentSkillConfig{{Type: ptr("anthropic"), SkillID: ptr("sk1")}},
		}},
	}
	ctx := &AgentConversionContext{System: "be helpful"}
	p := r.ToAnthropicNew(ctx)

	if p.Name != "my-agent" {
		t.Errorf("Name = %q, want %q", p.Name, "my-agent")
	}
	if p.Model.ID != "claude-opus-4-7" {
		t.Errorf("Model.ID = %q, want %q", p.Model.ID, "claude-opus-4-7")
	}
	if p.Description.Value != "desc" {
		t.Errorf("Description = %q, want %q", p.Description.Value, "desc")
	}
	if p.System.Value != "be helpful" {
		t.Errorf("System = %q, want %q", p.System.Value, "be helpful")
	}
	if len(p.MCPServers) != 1 || p.MCPServers[0].Name != "srv" {
		t.Errorf("MCPServers = %v", p.MCPServers)
	}
	if len(p.Skills) != 1 {
		t.Errorf("Skills len = %d, want 1", len(p.Skills))
	}
}

func TestAgentToAnthropicNewNilContext(t *testing.T) {
	r := &Agent{Spec: AgentSpec{ForProvider: AgentParameters{Name: ptr("a")}}}
	p := r.ToAnthropicNew(nil)
	if p.System.Value != "" {
		t.Errorf("expected empty system when ctx is nil, got %q", p.System.Value)
	}
}

func TestAgentToAnthropicUpdate(t *testing.T) {
	ver := int64(3)
	r := &Agent{
		Spec:   AgentSpec{ForProvider: AgentParameters{Name: ptr("updated")}},
		Status: AgentStatus{AtProvider: AgentObservation{Version: &ver}},
	}
	p := r.ToAnthropicUpdate(&AgentConversionContext{System: "sys"})
	if p.Version.Value != 3 {
		t.Errorf("Version = %d, want 3", p.Version.Value)
	}
	if p.Name.Value != "updated" {
		t.Errorf("Name = %q, want %q", p.Name.Value, "updated")
	}
}

func TestAgentToAnthropicNew_ModelEffort(t *testing.T) {
	r := &Agent{
		Spec: AgentSpec{ForProvider: AgentParameters{
			Name:        ptr("a"),
			Model:       ptr("claude-opus-4-7"),
			ModelEffort: ptr("high"),
		}},
	}
	p := r.ToAnthropicNew(nil)
	if p.Model.ID != "claude-opus-4-7" {
		t.Errorf("Model.ID = %q, want %q", p.Model.ID, "claude-opus-4-7")
	}
	if p.Model.Effort.OfBetaManagedAgentsModelConfigsEffortBetaManagedAgentsEffortLevel.Value != "high" {
		t.Errorf("Model.Effort = %q, want %q", p.Model.Effort.OfBetaManagedAgentsModelConfigsEffortBetaManagedAgentsEffortLevel.Value, "high")
	}
}

func TestAgentToAnthropicUpdate_ModelEffort(t *testing.T) {
	ver := int64(1)
	r := &Agent{
		Spec:   AgentSpec{ForProvider: AgentParameters{Name: ptr("a"), ModelEffort: ptr("low")}},
		Status: AgentStatus{AtProvider: AgentObservation{Version: &ver}},
	}
	p := r.ToAnthropicUpdate(nil)
	if p.Model.Effort.OfBetaManagedAgentsModelConfigsEffortBetaManagedAgentsEffortLevel.Value != "low" {
		t.Errorf("Model.Effort = %q, want %q", p.Model.Effort.OfBetaManagedAgentsModelConfigsEffortBetaManagedAgentsEffortLevel.Value, "low")
	}
}

func TestAgentToAnthropicNew_MCPTool(t *testing.T) {
	r := &Agent{
		Spec: AgentSpec{ForProvider: AgentParameters{
			Name: ptr("a"),
			Tools: []AgentToolConfig{
				{Type: ptr("mcp_toolset"), MCPServerName: ptr("my-mcp-server")},
			},
		}},
	}
	p := r.ToAnthropicNew(nil)
	if len(p.Tools) != 1 {
		t.Fatalf("Tools len = %d, want 1", len(p.Tools))
	}
	tool := p.Tools[0]
	if tool.OfMCPToolset == nil {
		t.Fatalf("OfMCPToolset is nil, got %+v", tool)
	}
	if tool.OfMCPToolset.MCPServerName != "my-mcp-server" {
		t.Errorf("MCPServerName = %q, want %q", tool.OfMCPToolset.MCPServerName, "my-mcp-server")
	}
}

func TestAgentToAnthropicNew_CustomTool(t *testing.T) {
	r := &Agent{
		Spec: AgentSpec{ForProvider: AgentParameters{
			Name: ptr("a"),
			Tools: []AgentToolConfig{
				{
					Type:        ptr("custom"),
					Name:        ptr("my-tool"),
					Description: ptr("does stuff"),
					InputSchema: &AgentCustomToolInputSchema{
						Properties: runtime.RawExtension{Raw: []byte(`{"arg":{"type":"string"}}`)},
						Required:   []string{"arg"},
					},
				},
			},
		}},
	}
	p := r.ToAnthropicNew(nil)
	if len(p.Tools) != 1 {
		t.Fatalf("Tools len = %d, want 1", len(p.Tools))
	}
	tool := p.Tools[0]
	if tool.OfCustom == nil {
		t.Fatalf("OfCustom is nil, got %+v", tool)
	}
	if tool.OfCustom.Name != "my-tool" {
		t.Errorf("Name = %q, want %q", tool.OfCustom.Name, "my-tool")
	}
	if tool.OfCustom.Description != "does stuff" {
		t.Errorf("Description = %q, want %q", tool.OfCustom.Description, "does stuff")
	}
}

func TestAgentToAnthropicUpdate_MCPTool(t *testing.T) {
	ver := int64(1)
	r := &Agent{
		Spec:   AgentSpec{ForProvider: AgentParameters{Name: ptr("a"), Tools: []AgentToolConfig{{Type: ptr("mcp_toolset"), MCPServerName: ptr("srv")}}}},
		Status: AgentStatus{AtProvider: AgentObservation{Version: &ver}},
	}
	p := r.ToAnthropicUpdate(nil)
	if len(p.Tools) != 1 || p.Tools[0].OfMCPToolset == nil {
		t.Fatalf("expected OfMCPToolset, got %+v", p.Tools)
	}
	if p.Tools[0].OfMCPToolset.MCPServerName != "srv" {
		t.Errorf("MCPServerName = %q, want %q", p.Tools[0].OfMCPToolset.MCPServerName, "srv")
	}
}

func TestAgentToAnthropicUpdate_CustomTool(t *testing.T) {
	ver := int64(1)
	r := &Agent{
		Spec: AgentSpec{ForProvider: AgentParameters{
			Name: ptr("a"),
			Tools: []AgentToolConfig{{
				Type: ptr("custom"), Name: ptr("t"), Description: ptr("d"),
				InputSchema: &AgentCustomToolInputSchema{},
			}},
		}},
		Status: AgentStatus{AtProvider: AgentObservation{Version: &ver}},
	}
	p := r.ToAnthropicUpdate(nil)
	if len(p.Tools) != 1 || p.Tools[0].OfCustom == nil {
		t.Fatalf("expected OfCustom, got %+v", p.Tools)
	}
}

func TestAgentFromAnthropicObservation_MCPTool(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	resp := anthropic.BetaManagedAgentsAgent{
		ID: "x", Name: "n", Version: 1,
		CreatedAt: now, UpdatedAt: now,
		Model: anthropic.BetaManagedAgentsModelConfig{ID: "claude-opus-4-7"},
		Tools: []anthropic.BetaManagedAgentsAgentToolUnion{
			{Type: "mcp_toolset", MCPServerName: "my-srv"},
		},
	}
	r := &Agent{}
	r.FromAnthropicObservation(resp)
	if len(r.Status.AtProvider.Tools) != 1 {
		t.Fatalf("Tools len = %d, want 1", len(r.Status.AtProvider.Tools))
	}
	tool := r.Status.AtProvider.Tools[0]
	if tool.MCPServerName == nil || *tool.MCPServerName != "my-srv" {
		t.Errorf("MCPServerName = %v, want %q", tool.MCPServerName, "my-srv")
	}
}

func TestAgentFromAnthropicObservation_CustomTool(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	resp := anthropic.BetaManagedAgentsAgent{
		ID: "x", Name: "n", Version: 1,
		CreatedAt: now, UpdatedAt: now,
		Model: anthropic.BetaManagedAgentsModelConfig{ID: "claude-opus-4-7"},
		Tools: []anthropic.BetaManagedAgentsAgentToolUnion{
			{Type: "custom", Name: "my-tool", Description: "desc"},
		},
	}
	r := &Agent{}
	r.FromAnthropicObservation(resp)
	if len(r.Status.AtProvider.Tools) != 1 {
		t.Fatalf("Tools len = %d, want 1", len(r.Status.AtProvider.Tools))
	}
	tool := r.Status.AtProvider.Tools[0]
	if tool.Name == nil || *tool.Name != "my-tool" {
		t.Errorf("Name = %v, want %q", tool.Name, "my-tool")
	}
	if tool.Description == nil || *tool.Description != "desc" {
		t.Errorf("Description = %v, want %q", tool.Description, "desc")
	}
}

func TestAgentFromAnthropicObservation(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	ver := int64(2)
	resp := anthropic.BetaManagedAgentsAgent{
		ID:          "agt_123",
		Name:        "my-agent",
		Description: "desc",
		System:      "sys",
		Version:     ver,
		Metadata:    map[string]string{"k": "v"},
		CreatedAt:   now,
		UpdatedAt:   now,
		Model: anthropic.BetaManagedAgentsModelConfig{
			ID:     "claude-opus-4-7",
			Effort: anthropic.BetaManagedAgentsModelConfigEffortUnion{Type: "high"},
		},
		MCPServers: []anthropic.BetaManagedAgentsMCPServerURLDefinition{
			{Name: "srv", URL: "https://mcp.example"},
		},
		Skills: []anthropic.BetaManagedAgentsAgentSkillUnion{
			{SkillID: "sk1", Type: "anthropic"},
		},
		Tools: []anthropic.BetaManagedAgentsAgentToolUnion{
			{Type: "agent_toolset_20260401"},
		},
	}

	r := &Agent{}
	r.FromAnthropicObservation(resp)

	if r.Status.AtProvider.ID == nil || *r.Status.AtProvider.ID != "agt_123" {
		t.Errorf("ID = %v", r.Status.AtProvider.ID)
	}
	if r.Status.AtProvider.Name == nil || *r.Status.AtProvider.Name != "my-agent" {
		t.Errorf("Name = %v", r.Status.AtProvider.Name)
	}
	if r.Status.AtProvider.Version == nil || *r.Status.AtProvider.Version != 2 {
		t.Errorf("Version = %v", r.Status.AtProvider.Version)
	}
	if r.Status.AtProvider.Model == nil || r.Status.AtProvider.Model.ID == nil || *r.Status.AtProvider.Model.ID != "claude-opus-4-7" {
		t.Errorf("Model = %v", r.Status.AtProvider.Model)
	}
	if r.Status.AtProvider.ModelEffort == nil || *r.Status.AtProvider.ModelEffort != "high" {
		t.Errorf("ModelEffort = %v, want %q", r.Status.AtProvider.ModelEffort, "high")
	}
	if r.Status.AtProvider.ArchivedAt != nil {
		t.Errorf("ArchivedAt should be nil, got %v", r.Status.AtProvider.ArchivedAt)
	}
	// System must not be stored in status — only its SHA-256 digest.
	wantHash := "518b67e652531c5fe7e25d6b2c3b4ef6224e7d90da2091967dd47eb082b26a19"
	if r.Status.AtProvider.SystemSha256 == nil || *r.Status.AtProvider.SystemSha256 != wantHash {
		t.Errorf("SystemSha256 = %v, want %q", r.Status.AtProvider.SystemSha256, wantHash)
	}
	if len(r.Status.AtProvider.MCPServers) != 1 || *r.Status.AtProvider.MCPServers[0].Name != "srv" {
		t.Errorf("MCPServers = %v", r.Status.AtProvider.MCPServers)
	}
	if len(r.Status.AtProvider.Skills) != 1 || *r.Status.AtProvider.Skills[0].Type != "anthropic" {
		t.Errorf("Skills = %v", r.Status.AtProvider.Skills)
	}
	if len(r.Status.AtProvider.Tools) != 1 || *r.Status.AtProvider.Tools[0].Type != "agent_toolset_20260401" {
		t.Errorf("Tools = %v", r.Status.AtProvider.Tools)
	}
	wantCreated := now.Format(time.RFC3339)
	if r.Status.AtProvider.CreatedAt == nil || *r.Status.AtProvider.CreatedAt != wantCreated {
		t.Errorf("CreatedAt = %v, want %q", r.Status.AtProvider.CreatedAt, wantCreated)
	}
}

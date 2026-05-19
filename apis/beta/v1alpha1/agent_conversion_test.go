package v1alpha1_test

import (
	"testing"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"

	. "github.com/jonasz-lasut/provider-anthropic/apis/beta/v1alpha1"
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
	if p.Version != 3 {
		t.Errorf("Version = %d, want 3", p.Version)
	}
	if p.Name.Value != "updated" {
		t.Errorf("Name = %q, want %q", p.Name.Value, "updated")
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
		Model:       anthropic.BetaManagedAgentsModelConfig{ID: "claude-opus-4-7"},
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
	if r.Status.AtProvider.ArchivedAt != nil {
		t.Errorf("ArchivedAt should be nil, got %v", r.Status.AtProvider.ArchivedAt)
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

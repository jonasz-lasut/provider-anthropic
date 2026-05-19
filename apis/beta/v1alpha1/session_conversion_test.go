package v1alpha1_test

import (
	"testing"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"

	. "github.com/jonasz-lasut/provider-anthropic-platform/apis/beta/v1alpha1"
)

func TestSessionToAnthropicNew(t *testing.T) {
	agentID := "agt_123"
	r := &Session{
		Spec: SessionSpec{ForProvider: SessionParameters{
			AgentID:  &agentID,
			Title:    ptr("my-session"),
			Metadata: map[string]string{"k": "v"},
			VaultIDs: []string{"vlt_1"},
		}},
	}
	p := r.ToAnthropicNew()
	if p.Title.Value != "my-session" {
		t.Errorf("Title = %q, want %q", p.Title.Value, "my-session")
	}
	if len(p.VaultIDs) != 1 || p.VaultIDs[0] != "vlt_1" {
		t.Errorf("VaultIDs = %v", p.VaultIDs)
	}
}

func TestSessionToAnthropicUpdate(t *testing.T) {
	r := &Session{
		Spec: SessionSpec{ForProvider: SessionParameters{
			Title:    ptr("renamed"),
			Metadata: map[string]string{"x": "y"},
		}},
	}
	p := r.ToAnthropicUpdate()
	if p.Title.Value != "renamed" {
		t.Errorf("Title = %q, want %q", p.Title.Value, "renamed")
	}
	if p.Metadata["x"] != "y" {
		t.Errorf("Metadata = %v", p.Metadata)
	}
}

func TestSessionFromAnthropicObservation(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	resp := anthropic.BetaManagedAgentsSession{
		ID:            "sess_abc",
		Title:         "my-session",
		EnvironmentID: "env_1",
		Metadata:      map[string]string{"k": "v"},
		VaultIDs:      []string{"vlt_1"},
		Status:        anthropic.BetaManagedAgentsSessionStatusRunning,
		CreatedAt:     now,
		UpdatedAt:     now,
		Agent:         anthropic.BetaManagedAgentsSessionAgent{ID: "agt_123"},
	}
	r := &Session{}
	r.FromAnthropicObservation(resp)

	if r.Status.AtProvider.ID == nil || *r.Status.AtProvider.ID != "sess_abc" {
		t.Errorf("ID = %v", r.Status.AtProvider.ID)
	}
	if r.Status.AtProvider.AgentID == nil || *r.Status.AtProvider.AgentID != "agt_123" {
		t.Errorf("AgentID = %v", r.Status.AtProvider.AgentID)
	}
	if r.Status.AtProvider.EnvironmentID == nil || *r.Status.AtProvider.EnvironmentID != "env_1" {
		t.Errorf("EnvironmentID = %v", r.Status.AtProvider.EnvironmentID)
	}
	if r.Status.AtProvider.Status == nil || *r.Status.AtProvider.Status != "running" {
		t.Errorf("Status = %v", r.Status.AtProvider.Status)
	}
	if len(r.Status.AtProvider.VaultIDs) != 1 || r.Status.AtProvider.VaultIDs[0] != "vlt_1" {
		t.Errorf("VaultIDs = %v", r.Status.AtProvider.VaultIDs)
	}
}

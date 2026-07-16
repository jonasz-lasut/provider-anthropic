package v1beta1_test

import (
	"testing"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"

	. "github.com/jonasz-lasut/provider-anthropic/apis/managedagents/v1beta1"
)

func TestSessionToAnthropicNew_BasicFields(t *testing.T) {
	agentID := "agt_123"
	r := &Session{
		Spec: SessionSpec{ForProvider: SessionParameters{
			AgentID:  &agentID,
			Title:    ptr("my-session"),
			Metadata: map[string]string{"k": "v"},
			VaultIDs: []string{"vlt_1"},
		}},
	}
	p := r.ToAnthropicNew(&SessionConversionContext{})
	if p.Title.Value != "my-session" {
		t.Errorf("Title = %q, want %q", p.Title.Value, "my-session")
	}
	if len(p.VaultIDs) != 1 || p.VaultIDs[0] != "vlt_1" {
		t.Errorf("VaultIDs = %v", p.VaultIDs)
	}
}

func TestSessionToAnthropicNew_GitHubRepositoryUsesResolvedToken(t *testing.T) {
	r := &Session{
		Spec: SessionSpec{ForProvider: SessionParameters{
			Resources: []SessionResource{{
				Type: ptr("github_repository"),
				URL:  ptr("https://github.com/org/repo"),
				AuthorizationTokenSecretRef: &xpv2.LocalSecretKeySelector{
					LocalSecretReference: xpv2.LocalSecretReference{Name: "tok"},
					Key:                  "token",
				},
			}},
		}},
	}
	cctx := &SessionConversionContext{ResourceTokens: []string{"ghp_resolved"}}
	p := r.ToAnthropicNew(cctx)
	if len(p.Resources) != 1 {
		t.Fatalf("Resources len = %d, want 1", len(p.Resources))
	}
	gh := p.Resources[0].OfGitHubRepository
	if gh == nil {
		t.Fatalf("Resources[0].OfGitHubRepository is nil")
	}
	if gh.AuthorizationToken != "ghp_resolved" {
		t.Errorf("AuthorizationToken = %q, want ghp_resolved", gh.AuthorizationToken)
	}
	if gh.URL != "https://github.com/org/repo" {
		t.Errorf("URL = %q", gh.URL)
	}
}

func TestSessionToAnthropicNew_NoTokenWhenContextEmpty(t *testing.T) {
	r := &Session{
		Spec: SessionSpec{ForProvider: SessionParameters{
			Resources: []SessionResource{{
				Type: ptr("github_repository"),
				URL:  ptr("https://github.com/org/repo"),
			}},
		}},
	}
	p := r.ToAnthropicNew(&SessionConversionContext{})
	gh := p.Resources[0].OfGitHubRepository
	if gh == nil {
		t.Fatalf("Resources[0].OfGitHubRepository is nil")
	}
	if gh.AuthorizationToken != "" {
		t.Errorf("AuthorizationToken = %q, want empty", gh.AuthorizationToken)
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
		DeploymentID:  "dpl_1",
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
	if r.Status.AtProvider.DeploymentID == nil || *r.Status.AtProvider.DeploymentID != "dpl_1" {
		t.Errorf("DeploymentID = %v", r.Status.AtProvider.DeploymentID)
	}
	if r.Status.AtProvider.Status == nil || *r.Status.AtProvider.Status != "running" {
		t.Errorf("Status = %v", r.Status.AtProvider.Status)
	}
	if len(r.Status.AtProvider.VaultIDs) != 1 || r.Status.AtProvider.VaultIDs[0] != "vlt_1" {
		t.Errorf("VaultIDs = %v", r.Status.AtProvider.VaultIDs)
	}
}


package v1alpha1_test

import (
	"testing"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"

	. "github.com/jonasz-lasut/provider-anthropic/apis/beta/v1alpha1"
)

func TestVaultCredentialToAnthropicNewStaticBearer(t *testing.T) {
	authType := "static_bearer"
	mcpURL := "https://mcp.example"
	r := &VaultCredential{
		Spec: VaultCredentialSpec{ForProvider: VaultCredentialParameters{
			DisplayName: ptr("my-cred"),
			Auth: VaultCredentialAuth{
				Type:         &authType,
				MCPServerURL: &mcpURL,
			},
		}},
	}
	ctx := &VaultCredentialConversionContext{BearerToken: "tok123"}
	p, err := r.ToAnthropicNew(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if p.DisplayName.Value != "my-cred" {
		t.Errorf("DisplayName = %q", p.DisplayName.Value)
	}
	if p.Auth.OfStaticBearer == nil {
		t.Fatalf("expected OfStaticBearer to be set")
	}
	if p.Auth.OfStaticBearer.Token != "tok123" {
		t.Errorf("Token = %q, want %q", p.Auth.OfStaticBearer.Token, "tok123")
	}
	if p.Auth.OfStaticBearer.MCPServerURL != "https://mcp.example" {
		t.Errorf("MCPServerURL = %q", p.Auth.OfStaticBearer.MCPServerURL)
	}
}

func TestVaultCredentialToAnthropicNewMCPOAuth(t *testing.T) {
	authType := "mcp_oauth"
	mcpURL := "https://mcp.example"
	clientID := "client1"
	tokenEndpoint := "https://auth.example/token"
	tepType := "client_secret_post"
	r := &VaultCredential{
		Spec: VaultCredentialSpec{ForProvider: VaultCredentialParameters{
			Auth: VaultCredentialAuth{
				Type:         &authType,
				MCPServerURL: &mcpURL,
				Refresh: &VaultCredentialRefresh{
					ClientID:      &clientID,
					TokenEndpoint: &tokenEndpoint,
					TokenEndpointAuth: VaultCredentialTokenEndpointAuth{
						Type: &tepType,
					},
				},
			},
		}},
	}
	ctx := &VaultCredentialConversionContext{
		AccessToken:  "access123",
		RefreshToken: "refresh456",
		ClientSecret: "secret789",
	}
	p, err := r.ToAnthropicNew(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if p.Auth.OfMCPOAuth == nil {
		t.Fatalf("expected OfMCPOAuth to be set")
	}
	if p.Auth.OfMCPOAuth.AccessToken != "access123" {
		t.Errorf("AccessToken = %q", p.Auth.OfMCPOAuth.AccessToken)
	}
	if p.Auth.OfMCPOAuth.Refresh.RefreshToken != "refresh456" {
		t.Errorf("RefreshToken = %q", p.Auth.OfMCPOAuth.Refresh.RefreshToken)
	}
}

func TestVaultCredentialFromAnthropicObservation(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	authType := "static_bearer"
	mcpURL := "https://mcp.example"
	resp := anthropic.BetaManagedAgentsCredential{
		ID:          "cred_abc",
		VaultID:     "vlt_1",
		DisplayName: "my-cred",
		Metadata:    map[string]string{"k": "v"},
		Auth: anthropic.BetaManagedAgentsCredentialAuthUnion{
			Type:         authType,
			MCPServerURL: mcpURL,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	r := &VaultCredential{}
	r.FromAnthropicObservation(resp)

	if r.Status.AtProvider.ID == nil || *r.Status.AtProvider.ID != "cred_abc" {
		t.Errorf("ID = %v", r.Status.AtProvider.ID)
	}
	if r.Status.AtProvider.VaultID == nil || *r.Status.AtProvider.VaultID != "vlt_1" {
		t.Errorf("VaultID = %v", r.Status.AtProvider.VaultID)
	}
	if r.Status.AtProvider.Auth == nil {
		t.Fatal("Auth is nil")
	}
	if r.Status.AtProvider.Auth.Type == nil || *r.Status.AtProvider.Auth.Type != "static_bearer" {
		t.Errorf("Auth.Type = %v", r.Status.AtProvider.Auth.Type)
	}
	if r.Status.AtProvider.Auth.MCPServerURL == nil || *r.Status.AtProvider.Auth.MCPServerURL != "https://mcp.example" {
		t.Errorf("Auth.MCPServerURL = %v", r.Status.AtProvider.Auth.MCPServerURL)
	}
	if r.Status.AtProvider.ArchivedAt != nil {
		t.Errorf("ArchivedAt should be nil, got %v", r.Status.AtProvider.ArchivedAt)
	}
}

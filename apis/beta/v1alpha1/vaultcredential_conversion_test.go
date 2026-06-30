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

func TestVaultCredentialToAnthropicNewEnvironmentVariableLimited(t *testing.T) {
	authType := "environment_variable"
	secretName := "API_KEY"
	netType := "limited"
	r := &VaultCredential{
		Spec: VaultCredentialSpec{ForProvider: VaultCredentialParameters{
			Auth: VaultCredentialAuth{
				Type:       &authType,
				SecretName: &secretName,
				Networking: &VaultCredentialNetworking{
					Type:         &netType,
					AllowedHosts: []string{"api.example.com", "*.internal.example.com"},
				},
				InjectionLocation: &VaultCredentialInjectionLocation{
					Body:   ptr(true),
					Header: ptr(false),
				},
			},
		}},
	}
	ctx := &VaultCredentialConversionContext{SecretValue: "s3cr3t"}
	p, err := r.ToAnthropicNew(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if p.Auth.OfEnvironmentVariable == nil {
		t.Fatalf("expected OfEnvironmentVariable to be set")
	}
	ev := p.Auth.OfEnvironmentVariable
	if ev.SecretName != "API_KEY" {
		t.Errorf("SecretName = %q", ev.SecretName)
	}
	if ev.SecretValue != "s3cr3t" {
		t.Errorf("SecretValue = %q", ev.SecretValue)
	}
	if ev.Networking.OfLimited == nil {
		t.Fatalf("expected OfLimited networking")
	}
	if len(ev.Networking.OfLimited.AllowedHosts) != 2 || ev.Networking.OfLimited.AllowedHosts[0] != "api.example.com" {
		t.Errorf("AllowedHosts = %v", ev.Networking.OfLimited.AllowedHosts)
	}
	if ev.InjectionLocation.Body.Value != true {
		t.Errorf("InjectionLocation.Body = %v, want true", ev.InjectionLocation.Body.Value)
	}
	if ev.InjectionLocation.Header.Value != false {
		t.Errorf("InjectionLocation.Header = %v, want false", ev.InjectionLocation.Header.Value)
	}
}

func TestVaultCredentialToAnthropicNewEnvironmentVariableUnrestricted(t *testing.T) {
	authType := "environment_variable"
	secretName := "TOKEN"
	netType := "unrestricted"
	r := &VaultCredential{
		Spec: VaultCredentialSpec{ForProvider: VaultCredentialParameters{
			Auth: VaultCredentialAuth{
				Type:       &authType,
				SecretName: &secretName,
				Networking: &VaultCredentialNetworking{Type: &netType},
			},
		}},
	}
	ctx := &VaultCredentialConversionContext{SecretValue: "v"}
	p, err := r.ToAnthropicNew(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if p.Auth.OfEnvironmentVariable == nil {
		t.Fatalf("expected OfEnvironmentVariable to be set")
	}
	if p.Auth.OfEnvironmentVariable.Networking.OfUnrestricted == nil {
		t.Fatalf("expected OfUnrestricted networking")
	}
}

func TestVaultCredentialToAnthropicUpdateEnvironmentVariable(t *testing.T) {
	authType := "environment_variable"
	netType := "unrestricted"
	r := &VaultCredential{
		Spec: VaultCredentialSpec{ForProvider: VaultCredentialParameters{
			Auth: VaultCredentialAuth{
				Type:       &authType,
				Networking: &VaultCredentialNetworking{Type: &netType},
				InjectionLocation: &VaultCredentialInjectionLocation{
					Header: ptr(true),
				},
			},
		}},
	}
	ctx := &VaultCredentialConversionContext{SecretValue: "newsecret"}
	p, err := r.ToAnthropicUpdate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if p.Auth.OfEnvironmentVariable == nil {
		t.Fatalf("expected OfEnvironmentVariable to be set")
	}
	ev := p.Auth.OfEnvironmentVariable
	if ev.SecretValue.Value != "newsecret" {
		t.Errorf("SecretValue = %q", ev.SecretValue.Value)
	}
	if ev.Networking.OfUnrestricted == nil {
		t.Fatalf("expected OfUnrestricted networking")
	}
	if ev.InjectionLocation.Header.Value != true {
		t.Errorf("InjectionLocation.Header = %v, want true", ev.InjectionLocation.Header.Value)
	}
	if ev.InjectionLocation.Body.Valid() {
		t.Errorf("InjectionLocation.Body should be unset, got %v", ev.InjectionLocation.Body.Value)
	}
}

func TestVaultCredentialFromAnthropicObservationEnvironmentVariable(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	resp := anthropic.BetaManagedAgentsCredential{
		ID:          "cred_env",
		VaultID:     "vlt_1",
		DisplayName: "env-cred",
		Auth: anthropic.BetaManagedAgentsCredentialAuthUnion{
			Type:       "environment_variable",
			SecretName: "API_KEY",
			Networking: anthropic.BetaManagedAgentsEnvironmentVariableAuthResponseNetworkingUnion{
				Type:         "limited",
				AllowedHosts: []string{"api.example.com"},
			},
			InjectionLocation: anthropic.BetaManagedAgentsInjectionLocationResponse{
				Body:   true,
				Header: false,
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	r := &VaultCredential{}
	r.FromAnthropicObservation(resp)

	if r.Status.AtProvider.Auth == nil {
		t.Fatal("Auth is nil")
	}
	if r.Status.AtProvider.Auth.Type == nil || *r.Status.AtProvider.Auth.Type != "environment_variable" {
		t.Errorf("Auth.Type = %v", r.Status.AtProvider.Auth.Type)
	}
	if r.Status.AtProvider.Auth.SecretName == nil || *r.Status.AtProvider.Auth.SecretName != "API_KEY" {
		t.Errorf("Auth.SecretName = %v", r.Status.AtProvider.Auth.SecretName)
	}
	if r.Status.AtProvider.Auth.Networking == nil {
		t.Fatal("Auth.Networking is nil")
	}
	if r.Status.AtProvider.Auth.Networking.Type == nil || *r.Status.AtProvider.Auth.Networking.Type != "limited" {
		t.Errorf("Networking.Type = %v", r.Status.AtProvider.Auth.Networking.Type)
	}
	if len(r.Status.AtProvider.Auth.Networking.AllowedHosts) != 1 || r.Status.AtProvider.Auth.Networking.AllowedHosts[0] != "api.example.com" {
		t.Errorf("Networking.AllowedHosts = %v", r.Status.AtProvider.Auth.Networking.AllowedHosts)
	}
	if r.Status.AtProvider.Auth.InjectionLocation == nil {
		t.Fatal("Auth.InjectionLocation is nil")
	}
	if r.Status.AtProvider.Auth.InjectionLocation.Body == nil || *r.Status.AtProvider.Auth.InjectionLocation.Body != true {
		t.Errorf("InjectionLocation.Body = %v, want true", r.Status.AtProvider.Auth.InjectionLocation.Body)
	}
	if r.Status.AtProvider.Auth.InjectionLocation.Header == nil || *r.Status.AtProvider.Auth.InjectionLocation.Header != false {
		t.Errorf("InjectionLocation.Header = %v, want false", r.Status.AtProvider.Auth.InjectionLocation.Header)
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

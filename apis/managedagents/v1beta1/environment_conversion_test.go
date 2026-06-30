package v1beta1_test

import (
	"testing"

	anthropic "github.com/anthropics/anthropic-sdk-go"

	. "github.com/jonasz-lasut/provider-anthropic/apis/managedagents/v1beta1"
)

func TestEnvironmentToAnthropicNew(t *testing.T) {
	name := "my-env"
	desc := "test environment"
	netType := "unrestricted"
	r := &Environment{
		Spec: EnvironmentSpec{ForProvider: EnvironmentParameters{
			Name:        &name,
			Description: &desc,
			Metadata:    map[string]string{"k": "v"},
			Config: &EnvironmentCloudConfig{
				Networking: &EnvironmentNetworkingConfig{Type: &netType},
			},
		}},
	}
	p := r.ToAnthropicNew()
	if p.Name != "my-env" {
		t.Errorf("Name = %q", p.Name)
	}
	if p.Description.Value != "test environment" {
		t.Errorf("Description = %q", p.Description.Value)
	}
	if p.Config.OfCloud == nil || p.Config.OfCloud.Networking.OfUnrestricted == nil {
		t.Errorf("expected unrestricted networking")
	}
	if p.Metadata["k"] != "v" {
		t.Errorf("Metadata = %v", p.Metadata)
	}
}

func TestEnvironmentToAnthropicUpdate(t *testing.T) {
	name := "renamed"
	netType := "limited"
	allowMCP := true
	r := &Environment{
		Spec: EnvironmentSpec{ForProvider: EnvironmentParameters{
			Name:     &name,
			Metadata: map[string]string{"x": "y"},
			Config: &EnvironmentCloudConfig{
				Networking: &EnvironmentNetworkingConfig{
					Type:            &netType,
					AllowMCPServers: &allowMCP,
					AllowedHosts:    []string{"example.com"},
				},
			},
		}},
	}
	p := r.ToAnthropicUpdate()
	if p.Name.Value != "renamed" {
		t.Errorf("Name = %q", p.Name.Value)
	}
	if p.Metadata["x"] != "y" {
		t.Errorf("Metadata = %v", p.Metadata)
	}
	if p.Config.OfCloud == nil || p.Config.OfCloud.Networking.OfLimited == nil {
		t.Fatalf("expected limited networking")
	}
	if len(p.Config.OfCloud.Networking.OfLimited.AllowedHosts) != 1 {
		t.Errorf("AllowedHosts = %v", p.Config.OfCloud.Networking.OfLimited.AllowedHosts)
	}
}

func TestEnvironmentFromAnthropicObservation(t *testing.T) {
	resp := anthropic.BetaEnvironment{
		ID:          "env_abc",
		Name:        "my-env",
		Description: "desc",
		Metadata:    map[string]string{"k": "v"},
		CreatedAt:   "2025-01-01T00:00:00Z",
		UpdatedAt:   "2025-01-02T00:00:00Z",
		Config: anthropic.BetaEnvironmentConfigUnion{
			Networking: anthropic.BetaCloudConfigNetworkingUnion{
				Type:            "limited",
				AllowMCPServers: true,
				AllowedHosts:    []string{"example.com"},
			},
			Packages: anthropic.BetaPackages{
				Pip: []string{"requests"},
			},
		},
	}
	r := &Environment{}
	r.FromAnthropicObservation(resp)

	if r.Status.AtProvider.ID == nil || *r.Status.AtProvider.ID != "env_abc" {
		t.Errorf("ID = %v", r.Status.AtProvider.ID)
	}
	if r.Status.AtProvider.Name == nil || *r.Status.AtProvider.Name != "my-env" {
		t.Errorf("Name = %v", r.Status.AtProvider.Name)
	}
	if r.Status.AtProvider.Config == nil {
		t.Fatal("Config is nil")
	}
	if r.Status.AtProvider.Config.Networking == nil {
		t.Fatal("Config.Networking is nil")
	}
	if r.Status.AtProvider.Config.Networking.Type == nil || *r.Status.AtProvider.Config.Networking.Type != "limited" {
		t.Errorf("Networking.Type = %v", r.Status.AtProvider.Config.Networking.Type)
	}
	if r.Status.AtProvider.Config.Networking.AllowMCPServers == nil || !*r.Status.AtProvider.Config.Networking.AllowMCPServers {
		t.Errorf("AllowMCPServers = %v", r.Status.AtProvider.Config.Networking.AllowMCPServers)
	}
	if r.Status.AtProvider.Config.Packages == nil {
		t.Fatal("Config.Packages is nil")
	}
	if len(r.Status.AtProvider.Config.Packages.Pip) != 1 || r.Status.AtProvider.Config.Packages.Pip[0] != "requests" {
		t.Errorf("Packages.Pip = %v", r.Status.AtProvider.Config.Packages.Pip)
	}
	if r.Status.AtProvider.ArchivedAt != nil {
		t.Errorf("ArchivedAt should be nil, got %v", r.Status.AtProvider.ArchivedAt)
	}
}

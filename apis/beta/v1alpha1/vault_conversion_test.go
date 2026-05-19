package v1alpha1_test

import (
	"testing"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"

	. "github.com/jonasz-lasut/provider-anthropic-platform/apis/beta/v1alpha1"
)

func TestVaultToAnthropicNew(t *testing.T) {
	r := &Vault{
		Spec: VaultSpec{ForProvider: VaultParameters{
			DisplayName: ptr("my-vault"),
			Metadata:    map[string]string{"env": "prod"},
		}},
	}
	p := r.ToAnthropicNew()
	if p.DisplayName != "my-vault" {
		t.Errorf("DisplayName = %q, want %q", p.DisplayName, "my-vault")
	}
	if p.Metadata["env"] != "prod" {
		t.Errorf("Metadata = %v", p.Metadata)
	}
}

func TestVaultToAnthropicUpdate(t *testing.T) {
	r := &Vault{
		Spec: VaultSpec{ForProvider: VaultParameters{
			DisplayName: ptr("renamed"),
		}},
	}
	p := r.ToAnthropicUpdate()
	if p.DisplayName.Value != "renamed" {
		t.Errorf("DisplayName = %q, want %q", p.DisplayName.Value, "renamed")
	}
}

func TestVaultFromAnthropicObservation(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	resp := anthropic.BetaManagedAgentsVault{
		ID:          "vlt_abc",
		DisplayName: "my-vault",
		Metadata:    map[string]string{"env": "prod"},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	r := &Vault{}
	r.FromAnthropicObservation(resp)

	if r.Status.AtProvider.ID == nil || *r.Status.AtProvider.ID != "vlt_abc" {
		t.Errorf("ID = %v", r.Status.AtProvider.ID)
	}
	if r.Status.AtProvider.DisplayName == nil || *r.Status.AtProvider.DisplayName != "my-vault" {
		t.Errorf("DisplayName = %v", r.Status.AtProvider.DisplayName)
	}
	if r.Status.AtProvider.ArchivedAt != nil {
		t.Errorf("ArchivedAt should be nil, got %v", r.Status.AtProvider.ArchivedAt)
	}
	wantCreated := now.Format(time.RFC3339)
	if r.Status.AtProvider.CreatedAt == nil || *r.Status.AtProvider.CreatedAt != wantCreated {
		t.Errorf("CreatedAt = %v, want %q", r.Status.AtProvider.CreatedAt, wantCreated)
	}
}

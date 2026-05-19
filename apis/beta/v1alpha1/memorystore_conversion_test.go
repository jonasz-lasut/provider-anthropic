package v1alpha1_test

import (
	"testing"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"

	. "github.com/jonasz-lasut/provider-anthropic/apis/beta/v1alpha1"
)

func TestMemoryStoreToAnthropicNew(t *testing.T) {
	r := &MemoryStore{
		Spec: MemoryStoreSpec{ForProvider: MemoryStoreParameters{
			Name:        ptr("my-store"),
			Description: ptr("stores stuff"),
			Metadata:    map[string]string{"team": "ai"},
		}},
	}
	p := r.ToAnthropicNew()
	if p.Name != "my-store" {
		t.Errorf("Name = %q, want %q", p.Name, "my-store")
	}
	if p.Description.Value != "stores stuff" {
		t.Errorf("Description = %q, want %q", p.Description.Value, "stores stuff")
	}
}

func TestMemoryStoreToAnthropicUpdate(t *testing.T) {
	r := &MemoryStore{
		Spec: MemoryStoreSpec{ForProvider: MemoryStoreParameters{
			Name:        ptr("renamed"),
			Description: ptr("new desc"),
		}},
	}
	p := r.ToAnthropicUpdate()
	if p.Name.Value != "renamed" {
		t.Errorf("Name = %q, want %q", p.Name.Value, "renamed")
	}
	if p.Description.Value != "new desc" {
		t.Errorf("Description = %q", p.Description.Value)
	}
}

func TestMemoryStoreFromAnthropicObservation(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	resp := anthropic.BetaManagedAgentsMemoryStore{
		ID:          "memstore_abc",
		Name:        "my-store",
		Description: "stores stuff",
		Metadata:    map[string]string{"team": "ai"},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	r := &MemoryStore{}
	r.FromAnthropicObservation(resp)

	if r.Status.AtProvider.ID == nil || *r.Status.AtProvider.ID != "memstore_abc" {
		t.Errorf("ID = %v", r.Status.AtProvider.ID)
	}
	if r.Status.AtProvider.Name == nil || *r.Status.AtProvider.Name != "my-store" {
		t.Errorf("Name = %v", r.Status.AtProvider.Name)
	}
	if r.Status.AtProvider.Description == nil || *r.Status.AtProvider.Description != "stores stuff" {
		t.Errorf("Description = %v", r.Status.AtProvider.Description)
	}
	wantCreated := now.Format(time.RFC3339)
	if r.Status.AtProvider.CreatedAt == nil || *r.Status.AtProvider.CreatedAt != wantCreated {
		t.Errorf("CreatedAt = %v, want %q", r.Status.AtProvider.CreatedAt, wantCreated)
	}
}

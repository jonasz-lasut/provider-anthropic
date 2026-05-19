package v1alpha1_test

import (
	"testing"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"

	. "github.com/jonasz-lasut/provider-anthropic-platform/apis/beta/v1alpha1"
)

func TestMemoryStoreMemoryToAnthropicNew(t *testing.T) {
	r := &MemoryStoreMemory{
		Spec: MemoryStoreMemorySpec{ForProvider: MemoryStoreMemoryParameters{
			Path: ptr("/notes/foo.md"),
		}},
	}
	ctx := &MemoryStoreMemoryConversionContext{Content: "hello world"}
	p := r.ToAnthropicNew(ctx)
	if p.Path != "/notes/foo.md" {
		t.Errorf("Path = %q, want %q", p.Path, "/notes/foo.md")
	}
	if p.Content.Value != "hello world" {
		t.Errorf("Content = %q, want %q", p.Content.Value, "hello world")
	}
}

func TestMemoryStoreMemoryToAnthropicUpdate(t *testing.T) {
	storeID := "memstore_xyz"
	r := &MemoryStoreMemory{
		Spec: MemoryStoreMemorySpec{ForProvider: MemoryStoreMemoryParameters{
			MemoryStoreID: &storeID,
			Path:          ptr("/notes/bar.md"),
		}},
	}
	ctx := &MemoryStoreMemoryConversionContext{Content: "updated"}
	p := r.ToAnthropicUpdate(ctx)
	if p.MemoryStoreID != storeID {
		t.Errorf("MemoryStoreID = %q, want %q", p.MemoryStoreID, storeID)
	}
	if p.Path.Value != "/notes/bar.md" {
		t.Errorf("Path = %q", p.Path.Value)
	}
	if p.Content.Value != "updated" {
		t.Errorf("Content = %q", p.Content.Value)
	}
}

func TestMemoryStoreMemoryFromAnthropicObservation(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	contentSize := int64(11)
	resp := anthropic.BetaManagedAgentsMemory{
		ID:               "mem_123",
		MemoryStoreID:    "memstore_xyz",
		MemoryVersionID:  "memver_1",
		ContentSha256:    "abc123",
		ContentSizeBytes: contentSize,
		Path:             "/notes/foo.md",
		CreatedAt:        now,
		UpdatedAt:        now,
		Content:          "hello world",
	}
	r := &MemoryStoreMemory{}
	r.FromAnthropicObservation(resp)

	if r.Status.AtProvider.ID == nil || *r.Status.AtProvider.ID != "mem_123" {
		t.Errorf("ID = %v", r.Status.AtProvider.ID)
	}
	if r.Status.AtProvider.MemoryStoreID == nil || *r.Status.AtProvider.MemoryStoreID != "memstore_xyz" {
		t.Errorf("MemoryStoreID = %v", r.Status.AtProvider.MemoryStoreID)
	}
	if r.Status.AtProvider.ContentSha256 == nil || *r.Status.AtProvider.ContentSha256 != "abc123" {
		t.Errorf("ContentSha256 = %v", r.Status.AtProvider.ContentSha256)
	}
	if r.Status.AtProvider.ContentSizeBytes == nil || *r.Status.AtProvider.ContentSizeBytes != 11 {
		t.Errorf("ContentSizeBytes = %v", r.Status.AtProvider.ContentSizeBytes)
	}
	if r.Status.AtProvider.Path == nil || *r.Status.AtProvider.Path != "/notes/foo.md" {
		t.Errorf("Path = %v", r.Status.AtProvider.Path)
	}
}

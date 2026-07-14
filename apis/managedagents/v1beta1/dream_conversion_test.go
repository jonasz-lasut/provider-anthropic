package v1beta1_test

import (
	"testing"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"

	. "github.com/jonasz-lasut/provider-anthropic/apis/managedagents/v1beta1"
)

func TestDreamToAnthropicNew(t *testing.T) {
	r := &Dream{
		Spec: DreamSpec{ForProvider: DreamParameters{
			Inputs: []DreamInput{
				{Type: new("memory_store"), MemoryStoreID: new("mem_in")},
				{Type: new("sessions"), SessionIDs: []string{"sess_1", "sess_2"}},
			},
			Model:        &DreamModelConfig{ID: new("claude-opus-4-7"), Speed: new("fast")},
			Instructions: new("consolidate the notes"),
		}},
	}

	p := r.ToAnthropicNew()

	if p.Instructions.Value != "consolidate the notes" {
		t.Errorf("Instructions = %q, want %q", p.Instructions.Value, "consolidate the notes")
	}
	if p.Model.OfBetaDreamModelConfig == nil {
		t.Fatalf("Model.OfBetaDreamModelConfig is nil")
	}
	if p.Model.OfBetaDreamModelConfig.ID != "claude-opus-4-7" {
		t.Errorf("Model.ID = %q, want claude-opus-4-7", p.Model.OfBetaDreamModelConfig.ID)
	}
	if p.Model.OfBetaDreamModelConfig.Speed != anthropic.BetaDreamModelConfigParamSpeedFast {
		t.Errorf("Model.Speed = %q, want fast", p.Model.OfBetaDreamModelConfig.Speed)
	}
	if len(p.Inputs) != 2 {
		t.Fatalf("Inputs len = %d, want 2", len(p.Inputs))
	}
	ms := p.Inputs[0].OfMemoryStore
	if ms == nil {
		t.Fatalf("Inputs[0].OfMemoryStore is nil")
	}
	if ms.MemoryStoreID != "mem_in" {
		t.Errorf("Inputs[0].MemoryStoreID = %q, want mem_in", ms.MemoryStoreID)
	}
	ses := p.Inputs[1].OfSessions
	if ses == nil {
		t.Fatalf("Inputs[1].OfSessions is nil")
	}
	if len(ses.SessionIDs) != 2 || ses.SessionIDs[0] != "sess_1" || ses.SessionIDs[1] != "sess_2" {
		t.Errorf("Inputs[1].SessionIDs = %v, want [sess_1 sess_2]", ses.SessionIDs)
	}
}

func TestDreamToAnthropicNew_ModelWithoutSpeed(t *testing.T) {
	r := &Dream{
		Spec: DreamSpec{ForProvider: DreamParameters{
			Model: &DreamModelConfig{ID: new("claude-opus-4-7")},
		}},
	}

	p := r.ToAnthropicNew()

	if p.Model.OfBetaDreamModelConfig == nil {
		t.Fatalf("Model.OfBetaDreamModelConfig is nil")
	}
	if p.Model.OfBetaDreamModelConfig.Speed != "" {
		t.Errorf("Model.Speed = %q, want empty (unset)", p.Model.OfBetaDreamModelConfig.Speed)
	}
}

func TestDreamFromAnthropicObservation(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	resp := anthropic.BetaDream{
		ID:           "dream_abc",
		Instructions: "consolidate",
		SessionID:    "sess_root",
		Status:       anthropic.BetaDreamStatusCompleted,
		Model:        anthropic.BetaDreamModelConfig{ID: "claude-opus-4-7", Speed: anthropic.BetaDreamModelConfigSpeedFast},
		Inputs: []anthropic.BetaDreamInputUnion{
			{Type: "memory_store", MemoryStoreID: "mem_in"},
			{Type: "sessions", SessionIDs: []string{"sess_1"}},
		},
		Outputs: []anthropic.BetaDreamOutput{
			{Type: anthropic.BetaDreamOutputTypeMemoryStore, MemoryStoreID: "mem_out"},
		},
		Usage:     anthropic.BetaDreamUsage{InputTokens: 10, OutputTokens: 20, CacheReadInputTokens: 5, CacheCreationInputTokens: 3},
		CreatedAt: now,
		EndedAt:   now,
	}

	r := &Dream{}
	r.FromAnthropicObservation(resp)
	ap := r.Status.AtProvider

	if ap.ID == nil || *ap.ID != "dream_abc" {
		t.Errorf("ID = %v", ap.ID)
	}
	if ap.Status == nil || *ap.Status != "completed" {
		t.Errorf("Status = %v", ap.Status)
	}
	if ap.SessionID == nil || *ap.SessionID != "sess_root" {
		t.Errorf("SessionID = %v", ap.SessionID)
	}
	if ap.Model == nil || ap.Model.ID == nil || *ap.Model.ID != "claude-opus-4-7" {
		t.Errorf("Model.ID = %v", ap.Model)
	}
	if ap.Model == nil || ap.Model.Speed == nil || *ap.Model.Speed != "fast" {
		t.Errorf("Model.Speed = %v", ap.Model)
	}
	if len(ap.Inputs) != 2 {
		t.Fatalf("Inputs len = %d, want 2", len(ap.Inputs))
	}
	if ap.Inputs[0].MemoryStoreID == nil || *ap.Inputs[0].MemoryStoreID != "mem_in" {
		t.Errorf("Inputs[0].MemoryStoreID = %v", ap.Inputs[0].MemoryStoreID)
	}
	if len(ap.Inputs[1].SessionIDs) != 1 || ap.Inputs[1].SessionIDs[0] != "sess_1" {
		t.Errorf("Inputs[1].SessionIDs = %v", ap.Inputs[1].SessionIDs)
	}
	if len(ap.Outputs) != 1 || ap.Outputs[0].MemoryStoreID == nil || *ap.Outputs[0].MemoryStoreID != "mem_out" {
		t.Errorf("Outputs = %v", ap.Outputs)
	}
	if ap.Usage == nil || ap.Usage.InputTokens == nil || *ap.Usage.InputTokens != 10 {
		t.Errorf("Usage.InputTokens = %v", ap.Usage)
	}
	if ap.CreatedAt == nil || *ap.CreatedAt != now.Format(time.RFC3339) {
		t.Errorf("CreatedAt = %v", ap.CreatedAt)
	}
	if ap.EndedAt == nil || *ap.EndedAt != now.Format(time.RFC3339) {
		t.Errorf("EndedAt = %v", ap.EndedAt)
	}
}

func TestDreamFromAnthropicObservation_FailedPopulatesError(t *testing.T) {
	resp := anthropic.BetaDream{
		ID:     "dream_err",
		Status: anthropic.BetaDreamStatusFailed,
		Error:  anthropic.BetaDreamError{Message: "boom", Type: "internal_error"},
	}

	r := &Dream{}
	r.FromAnthropicObservation(resp)
	ap := r.Status.AtProvider

	if ap.Error == nil {
		t.Fatalf("Error is nil, want populated")
	}
	if ap.Error.Message == nil || *ap.Error.Message != "boom" {
		t.Errorf("Error.Message = %v", ap.Error.Message)
	}
	if ap.Error.Type == nil || *ap.Error.Type != "internal_error" {
		t.Errorf("Error.Type = %v", ap.Error.Type)
	}
}

func TestDreamFromAnthropicObservation_NoErrorWhenSucceeded(t *testing.T) {
	resp := anthropic.BetaDream{
		ID:     "dream_ok",
		Status: anthropic.BetaDreamStatusRunning,
	}

	r := &Dream{}
	r.FromAnthropicObservation(resp)

	if r.Status.AtProvider.Error != nil {
		t.Errorf("Error = %v, want nil", r.Status.AtProvider.Error)
	}
	if r.Status.AtProvider.EndedAt != nil {
		t.Errorf("EndedAt = %v, want nil (zero time)", r.Status.AtProvider.EndedAt)
	}
}

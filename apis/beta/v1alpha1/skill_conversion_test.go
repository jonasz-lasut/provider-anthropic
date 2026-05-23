/*
Copyright 2026 The provider-anthropic Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1_test

import (
	"testing"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	. "github.com/jonasz-lasut/provider-anthropic/apis/beta/v1alpha1"
)

func TestToAnthropicNew(t *testing.T) {
	sk := &Skill{}
	sk.Spec.ForProvider.DisplayTitle = ptr("My Skill")

	params := sk.ToAnthropicNew()

	if !params.DisplayTitle.Valid() || params.DisplayTitle.Value != "My Skill" {
		t.Errorf("expected DisplayTitle=My Skill, got %+v", params.DisplayTitle)
	}
	// Files are NOT set here — they are assembled by the reconciler.
	if len(params.Files) != 0 {
		t.Errorf("expected no Files in params, got %d", len(params.Files))
	}
}

func TestToAnthropicNew_NoDisplayTitle(t *testing.T) {
	sk := &Skill{}
	params := sk.ToAnthropicNew()
	if params.DisplayTitle.Valid() {
		t.Error("expected DisplayTitle to be unset")
	}
}

func TestToAnthropicNewVersion(t *testing.T) {
	sk := &Skill{}
	params := sk.ToAnthropicNewVersion()
	// Files are NOT set here — assembled by reconciler.
	if len(params.Files) != 0 {
		t.Errorf("expected no Files in params, got %d", len(params.Files))
	}
}

func TestFromAnthropicSkillObservation(t *testing.T) {
	sk := &Skill{}
	resp := anthropic.BetaSkillGetResponse{
		ID:            "skl_123",
		DisplayTitle:  "My Skill",
		Source:        "custom",
		CreatedAt:     "2026-01-01T00:00:00Z",
		UpdatedAt:     "2026-01-02T00:00:00Z",
		LatestVersion: "1759178010641129",
	}

	sk.FromAnthropicSkillObservation(resp)

	if sk.Status.AtProvider.ID == nil || *sk.Status.AtProvider.ID != "skl_123" {
		t.Errorf("ID mismatch: %v", sk.Status.AtProvider.ID)
	}
	if sk.Status.AtProvider.DisplayTitle == nil || *sk.Status.AtProvider.DisplayTitle != "My Skill" {
		t.Errorf("DisplayTitle mismatch")
	}
	if sk.Status.AtProvider.Source == nil || *sk.Status.AtProvider.Source != "custom" {
		t.Errorf("Source mismatch")
	}
	if sk.Status.AtProvider.LatestVersion == nil || *sk.Status.AtProvider.LatestVersion != "1759178010641129" {
		t.Errorf("LatestVersion mismatch")
	}
	if sk.Status.AtProvider.CreatedAt == nil || *sk.Status.AtProvider.CreatedAt != "2026-01-01T00:00:00Z" {
		t.Errorf("CreatedAt mismatch")
	}
	if sk.Status.AtProvider.UpdatedAt == nil || *sk.Status.AtProvider.UpdatedAt != "2026-01-02T00:00:00Z" {
		t.Errorf("UpdatedAt mismatch")
	}
}

func TestFromAnthropicVersionObservation(t *testing.T) {
	sk := &Skill{}
	resp := anthropic.BetaSkillVersionGetResponse{
		ID:          "skv_456",
		Name:        "my-skill",
		Description: "Does something useful",
		Directory:   "myskill",
		Version:     "1759178010641129",
		CreatedAt:   "2026-01-01T00:00:00Z",
	}

	sk.FromAnthropicVersionObservation(resp)

	if sk.Status.AtProvider.LatestVersionID == nil || *sk.Status.AtProvider.LatestVersionID != "skv_456" {
		t.Errorf("LatestVersionID mismatch")
	}
	if sk.Status.AtProvider.LatestVersionName == nil || *sk.Status.AtProvider.LatestVersionName != "my-skill" {
		t.Errorf("LatestVersionName mismatch")
	}
	if sk.Status.AtProvider.LatestVersionDescription == nil || *sk.Status.AtProvider.LatestVersionDescription != "Does something useful" {
		t.Errorf("LatestVersionDescription mismatch")
	}
	if sk.Status.AtProvider.LatestVersionDirectory == nil || *sk.Status.AtProvider.LatestVersionDirectory != "myskill" {
		t.Errorf("LatestVersionDirectory mismatch")
	}
	if sk.Status.AtProvider.LatestVersionCreatedAt == nil || *sk.Status.AtProvider.LatestVersionCreatedAt != "2026-01-01T00:00:00Z" {
		t.Errorf("LatestVersionCreatedAt mismatch")
	}
}

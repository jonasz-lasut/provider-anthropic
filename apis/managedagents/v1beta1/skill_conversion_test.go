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

package v1beta1_test

import (
	"testing"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	. "github.com/jonasz-lasut/provider-anthropic/apis/managedagents/v1beta1"
)

// optString compares param.Opt[string] by validity and value; the SDK keeps
// the omitted-or-valid state in unexported fields.
var optString = cmp.Comparer(func(a, b param.Opt[string]) bool {
	return a.Valid() == b.Valid() && a.Value == b.Value
})

func TestSkillToAnthropicNew(t *testing.T) {
	cases := map[string]struct {
		args SkillParameters
		want anthropic.BetaSkillNewParams
	}{
		"DisplayTitleSentAsDisplayName": {
			args: SkillParameters{DisplayTitle: new("My Skill")},
			want: anthropic.BetaSkillNewParams{DisplayName: anthropic.String("My Skill")},
		},
		"NoDisplayTitle": {
			args: SkillParameters{},
			want: anthropic.BetaSkillNewParams{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			sk := &Skill{Spec: SkillSpec{ForProvider: tc.args}}

			got := sk.ToAnthropicNew()

			// Files are never set by the conversion layer; the reconciler appends them.
			if diff := cmp.Diff(tc.want, got, optString, cmpopts.IgnoreUnexported(anthropic.BetaSkillNewParams{})); diff != "" {
				t.Errorf("ToAnthropicNew(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestSkillToAnthropicNewVersion(t *testing.T) {
	got := (&Skill{}).ToAnthropicNewVersion()

	// Files are never set by the conversion layer; the reconciler appends them.
	if diff := cmp.Diff(anthropic.BetaSkillVersionNewParams{}, got, optString, cmpopts.IgnoreUnexported(anthropic.BetaSkillVersionNewParams{})); diff != "" {
		t.Errorf("ToAnthropicNewVersion(): -want, +got:\n%s", diff)
	}
}

func TestSkillFromAnthropicSkillObservation(t *testing.T) {
	cases := map[string]struct {
		args anthropic.BetaSkill
		want SkillObservation
	}{
		"AllFields": {
			args: anthropic.BetaSkill{
				ID:              "skl_123",
				DisplayName:     "My Skill",
				Source:          anthropic.BetaSkillSource{Type: anthropic.BetaSkillSourceTypeCustom},
				CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt:       time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
				LatestVersionID: "skver_456",
			},
			want: SkillObservation{
				ID:              new("skl_123"),
				DisplayTitle:    new("My Skill"),
				Source:          new("custom"),
				CreatedAt:       new("2026-01-01T00:00:00Z"),
				UpdatedAt:       new("2026-01-02T00:00:00Z"),
				LatestVersionID: new("skver_456"),
			},
		},
		"PluginSource": {
			args: anthropic.BetaSkill{
				ID:     "skl_789",
				Source: anthropic.BetaSkillSource{Type: anthropic.BetaSkillSourceTypePlugin},
			},
			want: SkillObservation{
				ID:              new("skl_789"),
				DisplayTitle:    new(""),
				Source:          new("plugin"),
				CreatedAt:       new(time.Time{}.Format(time.RFC3339)),
				UpdatedAt:       new(time.Time{}.Format(time.RFC3339)),
				LatestVersionID: new(""),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			sk := &Skill{}

			sk.FromAnthropicSkillObservation(tc.args)

			if diff := cmp.Diff(tc.want, sk.Status.AtProvider); diff != "" {
				t.Errorf("FromAnthropicSkillObservation(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestSkillFromAnthropicVersionObservation(t *testing.T) {
	version := anthropic.BetaSkillVersion{
		ID:          "skver_456",
		SkillID:     "skl_123",
		Name:        "my-skill",
		Description: "Does something useful",
		CreatedAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	cases := map[string]struct {
		args struct {
			initial SkillObservation
			resp    anthropic.BetaSkillVersion
		}
		want SkillObservation
	}{
		"AllFields": {
			args: struct {
				initial SkillObservation
				resp    anthropic.BetaSkillVersion
			}{resp: version},
			want: SkillObservation{
				LatestVersionID:          new("skver_456"),
				LatestVersionName:        new("my-skill"),
				LatestVersionDescription: new("Does something useful"),
				LatestVersionCreatedAt:   new("2026-01-01T00:00:00Z"),
			},
		},
		"PreservesSkillLevelFields": {
			args: struct {
				initial SkillObservation
				resp    anthropic.BetaSkillVersion
			}{
				initial: SkillObservation{
					ID:              new("skl_123"),
					DisplayTitle:    new("My Skill"),
					LatestVersionID: new("skver_old"),
					FilesSha256:     new("deadbeef"),
				},
				resp: version,
			},
			want: SkillObservation{
				ID:                       new("skl_123"),
				DisplayTitle:             new("My Skill"),
				LatestVersionID:          new("skver_456"),
				LatestVersionName:        new("my-skill"),
				LatestVersionDescription: new("Does something useful"),
				LatestVersionCreatedAt:   new("2026-01-01T00:00:00Z"),
				FilesSha256:              new("deadbeef"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			sk := &Skill{Status: SkillStatus{AtProvider: tc.args.initial}}

			sk.FromAnthropicVersionObservation(tc.args.resp)

			if diff := cmp.Diff(tc.want, sk.Status.AtProvider); diff != "" {
				t.Errorf("FromAnthropicVersionObservation(): -want, +got:\n%s", diff)
			}
		})
	}
}

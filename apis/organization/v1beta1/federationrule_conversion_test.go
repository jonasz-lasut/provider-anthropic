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
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	. "github.com/jonasz-lasut/provider-anthropic/apis/organization/v1beta1"
)

var ignoreRuleParamInternals = cmpopts.IgnoreUnexported(
	anthropic.BetaOrganizationFederationRuleNewParams{},
	anthropic.BetaOrganizationFederationRuleUpdateParams{},
	anthropic.BetaFederationRuleMatchParam{},
	anthropic.BetaServiceAccountTargetParam{},
)

func TestFederationRuleToAnthropicNew(t *testing.T) {
	cases := map[string]struct {
		args FederationRuleParameters
		want anthropic.BetaOrganizationFederationRuleNewParams
	}{
		"AllFields": {
			args: FederationRuleParameters{
				Name: new("ci-rule"), Description: new("CI"), IssuerID: new("fdis_1"), ServiceAccountID: new("svac_1"),
				OAuthScope: new("workspace:developer"),
				Match:      &FederationRuleMatch{SubjectPrefix: new("repo:org/repo:*"), Claims: map[string]string{"repository": "org/repo"}, Audience: new("https://api.anthropic.com")},
				WorkspaceID: new("wrkspc_1"), AppliesToAllWorkspaces: new(false), TokenLifetimeSeconds: new(int64(600)),
			},
			want: anthropic.BetaOrganizationFederationRuleNewParams{
				Name: "ci-rule", Description: anthropic.String("CI"), IssuerID: "fdis_1", OAuthScope: "workspace:developer",
				Target: anthropic.BetaServiceAccountTargetParam{ServiceAccountID: "svac_1"},
				Match: anthropic.BetaFederationRuleMatchParam{
					SubjectPrefix: anthropic.String("repo:org/repo:*"), Claims: map[string]string{"repository": "org/repo"}, Audience: anthropic.String("https://api.anthropic.com"),
				},
				WorkspaceID: anthropic.String("wrkspc_1"), AppliesToAllWorkspaces: anthropic.Bool(false), TokenLifetimeSeconds: anthropic.Int(600),
			},
		},
		"AllWorkspaces": {
			args: FederationRuleParameters{
				Name: new("ci-rule"), IssuerID: new("fdis_1"), ServiceAccountID: new("svac_1"), OAuthScope: new("workspace:inference"),
				Match: &FederationRuleMatch{Condition: new("claims.env == 'prod'")}, AppliesToAllWorkspaces: new(true),
			},
			want: anthropic.BetaOrganizationFederationRuleNewParams{
				Name: "ci-rule", IssuerID: "fdis_1", OAuthScope: "workspace:inference",
				Target: anthropic.BetaServiceAccountTargetParam{ServiceAccountID: "svac_1"},
				Match:  anthropic.BetaFederationRuleMatchParam{Condition: anthropic.String("claims.env == 'prod'")},
				AppliesToAllWorkspaces: anthropic.Bool(true),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &FederationRule{Spec: FederationRuleSpec{ForProvider: tc.args}}

			got := r.ToAnthropicNew()

			if diff := cmp.Diff(tc.want, got, optString, optBool, optInt, ignoreRuleParamInternals); diff != "" {
				t.Errorf("ToAnthropicNew(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestFederationRuleToAnthropicUpdate(t *testing.T) {
	r := &FederationRule{Spec: FederationRuleSpec{ForProvider: FederationRuleParameters{
		Name: new("renamed"), IssuerID: new("fdis_1"), ServiceAccountID: new("svac_2"), OAuthScope: new("workspace:developer"),
		Match: &FederationRuleMatch{SubjectPrefix: new("repo:org/repo:*")}, TokenLifetimeSeconds: new(int64(900)),
	}}}
	want := anthropic.BetaOrganizationFederationRuleUpdateParams{
		Name: anthropic.String("renamed"), OAuthScope: anthropic.String("workspace:developer"),
		Target: anthropic.BetaServiceAccountTargetParam{ServiceAccountID: "svac_2"},
		Match:  anthropic.BetaFederationRuleMatchParam{SubjectPrefix: anthropic.String("repo:org/repo:*")},
		TokenLifetimeSeconds: anthropic.Int(900),
	}

	got := r.ToAnthropicUpdate()

	if diff := cmp.Diff(want, got, optString, optBool, optInt, ignoreRuleParamInternals); diff != "" {
		t.Errorf("ToAnthropicUpdate(): -want, +got:\n%s", diff)
	}
}

func TestFederationRuleFromAnthropicObservation(t *testing.T) {
	created := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	r := &FederationRule{}
	resp := anthropic.BetaFederationRule{
		ID: "fdrl_1", Name: "ci-rule", Description: "CI", IssuerID: "fdis_1", IssuerName: "gh", OAuthScope: "workspace:developer",
		Target: anthropic.BetaServiceAccountTarget{ServiceAccountID: "svac_1", ServiceAccountName: "ci-worker"},
		Match:  anthropic.BetaFederationRuleMatch{SubjectPrefix: "repo:org/repo:*", Claims: map[string]string{"repository": "org/repo"}},
		WorkspaceID: "wrkspc_1", WorkspaceIDs: []string{"wrkspc_1"}, AppliesToAllWorkspaces: false, TokenLifetimeSeconds: 3600,
		CreatedAt: created, UpdatedAt: created, ArchivedAt: created, CreatedByActorID: "user_1",
	}
	want := FederationRuleObservation{
		ID: new("fdrl_1"), Name: new("ci-rule"), Description: new("CI"), IssuerID: new("fdis_1"), IssuerName: new("gh"),
		ServiceAccountID: new("svac_1"), ServiceAccountName: new("ci-worker"), OAuthScope: new("workspace:developer"),
		Match:       &FederationRuleMatch{SubjectPrefix: new("repo:org/repo:*"), Claims: map[string]string{"repository": "org/repo"}},
		WorkspaceID: new("wrkspc_1"), WorkspaceIDs: []string{"wrkspc_1"}, AppliesToAllWorkspaces: new(false), TokenLifetimeSeconds: new(int64(3600)),
		CreatedAt: new("2026-09-08T10:00:00Z"), UpdatedAt: new("2026-09-08T10:00:00Z"), CreatedByActorID: new("user_1"),
	}

	r.FromAnthropicObservation(resp)

	if diff := cmp.Diff(want, r.Status.AtProvider); diff != "" {
		t.Errorf("FromAnthropicObservation(): -want, +got:\n%s", diff)
	}
}

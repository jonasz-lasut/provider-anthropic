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

func TestServiceAccountToAnthropicNew(t *testing.T) {
	cases := map[string]struct {
		args ServiceAccountParameters
		want anthropic.BetaOrganizationServiceAccountNewParams
	}{
		"AllFields": {
			args: ServiceAccountParameters{Name: new("ci-worker"), Description: new("CI"), OrganizationRole: new("developer")},
			want: anthropic.BetaOrganizationServiceAccountNewParams{
				Name:             "ci-worker",
				Description:      anthropic.String("CI"),
				OrganizationRole: anthropic.BetaOrganizationServiceAccountNewParamsOrganizationRoleDeveloper,
			},
		},
		"NameOnly": {
			args: ServiceAccountParameters{Name: new("ci-worker")},
			want: anthropic.BetaOrganizationServiceAccountNewParams{Name: "ci-worker"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &ServiceAccount{Spec: ServiceAccountSpec{ForProvider: tc.args}}

			got := r.ToAnthropicNew()

			if diff := cmp.Diff(tc.want, got, optString, cmpopts.IgnoreUnexported(anthropic.BetaOrganizationServiceAccountNewParams{})); diff != "" {
				t.Errorf("ToAnthropicNew(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestServiceAccountToAnthropicUpdate(t *testing.T) {
	r := &ServiceAccount{Spec: ServiceAccountSpec{ForProvider: ServiceAccountParameters{
		Name: new("ci-worker"), Description: new("renamed"), OrganizationRole: new("admin"),
	}}}
	want := anthropic.BetaOrganizationServiceAccountUpdateParams{
		Description:      anthropic.String("renamed"),
		OrganizationRole: anthropic.BetaOrganizationServiceAccountUpdateParamsOrganizationRoleAdmin,
	}

	got := r.ToAnthropicUpdate()

	if diff := cmp.Diff(want, got, optString, cmpopts.IgnoreUnexported(anthropic.BetaOrganizationServiceAccountUpdateParams{})); diff != "" {
		t.Errorf("ToAnthropicUpdate(): -want, +got:\n%s", diff)
	}
}

func TestServiceAccountFromAnthropicObservation(t *testing.T) {
	created := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	r := &ServiceAccount{}
	resp := anthropic.BetaServiceAccount{
		ID: "svac_1", Name: "ci-worker", Description: "CI", OrganizationRole: anthropic.BetaServiceAccountOrganizationRoleDeveloper,
		CreatedAt: created, UpdatedAt: created, ArchivedAt: created, CreatedByActorID: "user_1",
	}
	want := ServiceAccountObservation{
		ID: new("svac_1"), Name: new("ci-worker"), Description: new("CI"), OrganizationRole: new("developer"),
		CreatedAt: new("2026-09-08T10:00:00Z"), UpdatedAt: new("2026-09-08T10:00:00Z"), CreatedByActorID: new("user_1"),
	}

	r.FromAnthropicObservation(resp)

	if diff := cmp.Diff(want, r.Status.AtProvider); diff != "" {
		t.Errorf("FromAnthropicObservation(): -want, +got:\n%s", diff)
	}
}

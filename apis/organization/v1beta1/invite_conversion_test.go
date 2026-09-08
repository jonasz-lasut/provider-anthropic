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

func TestInviteToAnthropicNew(t *testing.T) {
	cases := map[string]struct {
		args InviteParameters
		want anthropic.BetaOrganizationInviteNewParams
	}{
		"EmailRoleGroups": {
			args: InviteParameters{Email: new("dev@example.com"), Role: new("developer"), RBACGroupIDs: []string{"grp_1"}},
			want: anthropic.BetaOrganizationInviteNewParams{
				Email:        "dev@example.com",
				Role:         anthropic.BetaOrganizationInviteNewParamsRoleDeveloper,
				RBACGroupIDs: []string{"grp_1"},
			},
		},
		"Empty": {
			args: InviteParameters{},
			want: anthropic.BetaOrganizationInviteNewParams{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &Invite{Spec: InviteSpec{ForProvider: tc.args}}

			got := r.ToAnthropicNew()

			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreUnexported(anthropic.BetaOrganizationInviteNewParams{})); diff != "" {
				t.Errorf("ToAnthropicNew(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestInviteFromAnthropicObservation(t *testing.T) {
	invited := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	expires := invited.Add(21 * 24 * time.Hour)

	cases := map[string]struct {
		args anthropic.BetaOrganizationInvite
		want InviteObservation
	}{
		"Pending": {
			args: anthropic.BetaOrganizationInvite{
				ID: "invite_1", Email: "dev@example.com", Role: anthropic.BetaOrganizationRoleDeveloper,
				Status: anthropic.BetaOrganizationInviteStatusPending, InvitedAt: invited, ExpiresAt: expires,
			},
			want: InviteObservation{
				ID: new("invite_1"), Email: new("dev@example.com"), Role: new("developer"), Status: new("pending"),
				InvitedAt: new("2026-09-08T10:00:00Z"), ExpiresAt: new("2026-09-29T10:00:00Z"),
			},
		},
		"AcceptedWithGroups": {
			args: anthropic.BetaOrganizationInvite{
				ID: "invite_2", Email: "dev@example.com", Role: anthropic.BetaOrganizationRoleUser,
				Status: anthropic.BetaOrganizationInviteStatusAccepted, InvitedAt: invited, ExpiresAt: expires,
				AcceptedAt: invited.Add(time.Hour), RBACGroupIDs: []string{"grp_1"},
			},
			want: InviteObservation{
				ID: new("invite_2"), Email: new("dev@example.com"), Role: new("user"), Status: new("accepted"),
				RBACGroupIDs: []string{"grp_1"},
				InvitedAt: new("2026-09-08T10:00:00Z"), ExpiresAt: new("2026-09-29T10:00:00Z"), AcceptedAt: new("2026-09-08T11:00:00Z"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &Invite{}

			r.FromAnthropicObservation(tc.args)

			if diff := cmp.Diff(tc.want, r.Status.AtProvider); diff != "" {
				t.Errorf("FromAnthropicObservation(): -want, +got:\n%s", diff)
			}
		})
	}
}

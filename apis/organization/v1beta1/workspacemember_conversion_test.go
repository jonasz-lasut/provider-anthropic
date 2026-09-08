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

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	. "github.com/jonasz-lasut/provider-anthropic/apis/organization/v1beta1"
)

func TestWorkspaceMemberToAnthropicNew(t *testing.T) {
	cases := map[string]struct {
		args WorkspaceMemberParameters
		want anthropic.BetaOrganizationWorkspaceMemberAddParams
	}{
		"UserAndRole": {
			args: WorkspaceMemberParameters{
				WorkspaceID:   new("wrkspc_1"),
				UserID:        new("user_1"),
				WorkspaceRole: new("workspace_developer"),
			},
			want: anthropic.BetaOrganizationWorkspaceMemberAddParams{
				UserID:        "user_1",
				WorkspaceRole: anthropic.BetaNoBillingWorkspaceRoleWorkspaceDeveloper,
			},
		},
		"Empty": {
			args: WorkspaceMemberParameters{},
			want: anthropic.BetaOrganizationWorkspaceMemberAddParams{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &WorkspaceMember{Spec: WorkspaceMemberSpec{ForProvider: tc.args}}

			got := r.ToAnthropicNew()

			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreUnexported(anthropic.BetaOrganizationWorkspaceMemberAddParams{})); diff != "" {
				t.Errorf("ToAnthropicNew(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestWorkspaceMemberToAnthropicUpdate(t *testing.T) {
	cases := map[string]struct {
		args WorkspaceMemberParameters
		want anthropic.BetaOrganizationWorkspaceMemberUpdateParams
	}{
		"WorkspaceAndRole": {
			args: WorkspaceMemberParameters{
				WorkspaceID:   new("wrkspc_1"),
				UserID:        new("user_1"),
				WorkspaceRole: new("workspace_admin"),
			},
			want: anthropic.BetaOrganizationWorkspaceMemberUpdateParams{
				WorkspaceID:   "wrkspc_1",
				WorkspaceRole: anthropic.BetaWorkspaceRoleWorkspaceAdmin,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &WorkspaceMember{Spec: WorkspaceMemberSpec{ForProvider: tc.args}}

			got := r.ToAnthropicUpdate()

			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreUnexported(anthropic.BetaOrganizationWorkspaceMemberUpdateParams{})); diff != "" {
				t.Errorf("ToAnthropicUpdate(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestWorkspaceMemberFromAnthropicObservation(t *testing.T) {
	cases := map[string]struct {
		args anthropic.BetaWorkspaceMember
		want WorkspaceMemberObservation
	}{
		"AllFields": {
			args: anthropic.BetaWorkspaceMember{
				UserID:        "user_1",
				WorkspaceID:   "wrkspc_1",
				WorkspaceRole: anthropic.BetaWorkspaceRoleWorkspaceBilling,
			},
			want: WorkspaceMemberObservation{
				UserID:        new("user_1"),
				WorkspaceID:   new("wrkspc_1"),
				WorkspaceRole: new("workspace_billing"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &WorkspaceMember{}

			r.FromAnthropicObservation(tc.args)

			if diff := cmp.Diff(tc.want, r.Status.AtProvider); diff != "" {
				t.Errorf("FromAnthropicObservation(): -want, +got:\n%s", diff)
			}
		})
	}
}

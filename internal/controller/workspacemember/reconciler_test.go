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

package workspacemember

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"

	v1beta1 "github.com/jonasz-lasut/provider-anthropic/apis/organization/v1beta1"
)

const (
	memberPath    = "/v1/organizations/workspaces/wrkspc_1/members/user_1"
	workspacePath = "/v1/organizations/workspaces/wrkspc_1"

	apiError = `{"type":"error","error":{"type":"invalid_request_error","message":"stub"}}`

	memberBody = `{"type":"workspace_member","user_id":"user_1","workspace_id":"wrkspc_1","workspace_role":"workspace_developer"}`

	activeWorkspace   = `{"id":"wrkspc_1","type":"workspace","name":"w","archived_at":null,"created_at":"2026-09-08T10:00:00Z","display_color":"","compartment_id":"c","external_key_id":"","tags":{},"data_residency":{"allowed_inference_geos":"unrestricted","default_inference_geo":"global","workspace_geo":"us"}}`
	archivedWorkspace = `{"id":"wrkspc_1","type":"workspace","name":"w","archived_at":"2026-09-08T11:00:00Z","created_at":"2026-09-08T10:00:00Z","display_color":"","compartment_id":"c","external_key_id":"","tags":{},"data_residency":{"allowed_inference_geos":"unrestricted","default_inference_geo":"global","workspace_geo":"us"}}`
)

type stubResponse struct {
	status int
	body   string
}

// stubClient routes requests by "METHOD path" to canned responses and records
// the paths it served, so a test can assert which calls the reconciler made.
func stubClient(t *testing.T, routes map[string]stubResponse, calls *[]string) *anthropic.Client {
	t.Helper()
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")
	c := anthropic.NewClient(
		option.WithAPIKey("sk-ant-test"),
		option.WithMaxRetries(0),
		option.WithMiddleware(func(req *http.Request, _ option.MiddlewareNext) (*http.Response, error) {
			key := req.Method + " " + req.URL.Path
			*calls = append(*calls, key)
			r, ok := routes[key]
			if !ok {
				t.Fatalf("unexpected request %s", key)
			}
			return &http.Response{
				StatusCode: r.status,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(r.body)),
				Request:    req,
			}, nil
		}),
	)
	return &c
}

func member() *v1beta1.WorkspaceMember {
	m := &v1beta1.WorkspaceMember{
		ObjectMeta: metav1.ObjectMeta{Name: "member", Namespace: "ns"},
		Spec: v1beta1.WorkspaceMemberSpec{ForProvider: v1beta1.WorkspaceMemberParameters{
			WorkspaceID:   new("wrkspc_1"),
			UserID:        new("user_1"),
			WorkspaceRole: new("workspace_developer"),
		}},
	}
	meta.SetExternalName(m, "user_1")
	return m
}

func TestDelete(t *testing.T) {
	type want struct {
		err   bool
		calls []string
	}
	cases := map[string]struct {
		args map[string]stubResponse
		want want
	}{
		"Removed": {
			args: map[string]stubResponse{
				"DELETE " + memberPath: {status: http.StatusOK, body: `{"type":"workspace_member_deleted","user_id":"user_1","workspace_id":"wrkspc_1"}`},
			},
			want: want{calls: []string{"DELETE " + memberPath}},
		},
		"AlreadyGone": {
			args: map[string]stubResponse{
				"DELETE " + memberPath: {status: http.StatusNotFound, body: apiError},
			},
			want: want{calls: []string{"DELETE " + memberPath}},
		},
		"ParentArchived": {
			args: map[string]stubResponse{
				"DELETE " + memberPath: {status: http.StatusBadRequest, body: apiError},
				"GET " + workspacePath: {status: http.StatusOK, body: archivedWorkspace},
			},
			want: want{calls: []string{"DELETE " + memberPath, "GET " + workspacePath}},
		},
		"ParentMissing": {
			args: map[string]stubResponse{
				"DELETE " + memberPath: {status: http.StatusBadRequest, body: apiError},
				"GET " + workspacePath: {status: http.StatusNotFound, body: apiError},
			},
			want: want{calls: []string{"DELETE " + memberPath, "GET " + workspacePath}},
		},
		"ParentActiveSurfacesError": {
			args: map[string]stubResponse{
				"DELETE " + memberPath: {status: http.StatusBadRequest, body: apiError},
				"GET " + workspacePath: {status: http.StatusOK, body: activeWorkspace},
			},
			want: want{err: true, calls: []string{"DELETE " + memberPath, "GET " + workspacePath}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var calls []string
			e := &external{client: stubClient(t, tc.args, &calls)}

			_, err := e.Delete(context.Background(), member())

			if (err != nil) != tc.want.err {
				t.Errorf("Delete() error = %v, want error: %t", err, tc.want.err)
			}
			if diff := cmp.Diff(tc.want.calls, calls); diff != "" {
				t.Errorf("API calls: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestObserve(t *testing.T) {
	type want struct {
		err      bool
		exists   bool
		upToDate bool
		role     *string
	}
	cases := map[string]struct {
		args map[string]stubResponse
		want want
	}{
		"MemberMatchesSpec": {
			args: map[string]stubResponse{"GET " + memberPath: {status: http.StatusOK, body: memberBody}},
			want: want{exists: true, upToDate: true, role: new("workspace_developer")},
		},
		"RoleDrifted": {
			args: map[string]stubResponse{"GET " + memberPath: {status: http.StatusOK, body: strings.Replace(memberBody, "workspace_developer", "workspace_user", 1)}},
			want: want{exists: true, upToDate: false, role: new("workspace_user")},
		},
		"NotAMember": {
			args: map[string]stubResponse{"GET " + memberPath: {status: http.StatusNotFound, body: apiError}},
			want: want{exists: false},
		},
		"ParentArchivedAnswers400": {
			args: map[string]stubResponse{
				"GET " + memberPath:    {status: http.StatusBadRequest, body: apiError},
				"GET " + workspacePath: {status: http.StatusOK, body: archivedWorkspace},
			},
			want: want{exists: false},
		},
		"ParentMissingAnswers400": {
			args: map[string]stubResponse{
				"GET " + memberPath:    {status: http.StatusBadRequest, body: apiError},
				"GET " + workspacePath: {status: http.StatusNotFound, body: apiError},
			},
			want: want{exists: false},
		},
		"ParentActiveSurfacesError": {
			args: map[string]stubResponse{
				"GET " + memberPath:    {status: http.StatusBadRequest, body: apiError},
				"GET " + workspacePath: {status: http.StatusOK, body: activeWorkspace},
			},
			want: want{err: true},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var calls []string
			e := &external{client: stubClient(t, tc.args, &calls)}
			m := member()

			obs, err := e.Observe(context.Background(), m)

			got := want{err: err != nil, exists: obs.ResourceExists, upToDate: obs.ResourceUpToDate, role: m.Status.AtProvider.WorkspaceRole}
			if diff := cmp.Diff(tc.want, got, cmp.AllowUnexported(want{})); diff != "" {
				t.Errorf("Observe(): -want, +got:\n%s", diff)
			}
		})
	}
}

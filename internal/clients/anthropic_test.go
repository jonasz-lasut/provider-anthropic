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

package clients

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/google/go-cmp/cmp"

	pcv1beta1 "github.com/jonasz-lasut/provider-anthropic/apis/config/v1beta1"
)

func TestAPIKeyFromCredentials(t *testing.T) {
	tests := []struct {
		name            string
		data            string
		identityType    pcv1beta1.IdentityType
		want            string
		wantErr         bool
		wantErrContains string
	}{
		{
			name:         "APIKey success",
			data:         `{"api_key":"sk-ant-abc123"}`,
			identityType: pcv1beta1.IdentityTypeAPIKey,
			want:         "sk-ant-abc123",
		},
		{
			name:            "APIKey missing api_key field",
			data:            `{"something_else":"value"}`,
			identityType:    pcv1beta1.IdentityTypeAPIKey,
			wantErr:         true,
			wantErrContains: "api_key",
		},
		{
			name:            "APIKey empty api_key value",
			data:            `{"api_key":""}`,
			identityType:    pcv1beta1.IdentityTypeAPIKey,
			wantErr:         true,
			wantErrContains: "api_key",
		},
		{
			name:            "invalid JSON",
			data:            `not-json`,
			identityType:    pcv1beta1.IdentityTypeAPIKey,
			wantErr:         true,
			wantErrContains: "unmarshal",
		},
		{
			name:            "unknown identity type",
			data:            `{"api_key":"sk-ant-abc123"}`,
			identityType:    pcv1beta1.IdentityType("Bogus"),
			wantErr:         true,
			wantErrContains: "Bogus",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := apiKeyFromCredentials([]byte(tc.data), tc.identityType)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (result %q)", got)
				}
				if tc.wantErrContains != "" && !strings.Contains(err.Error(), tc.wantErrContains) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErrContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClientOptions(t *testing.T) {
	type args struct {
		apiKey      string
		workspaceID *string
	}
	type headers struct {
		APIKey      string
		WorkspaceID string
	}
	cases := map[string]struct {
		args args
		want headers
	}{
		"WorkspaceIDSet": {
			args: args{apiKey: "sk-ant-abc123", workspaceID: new("wrkspc_01JwQvzr7rXLA5AGx3HKfFUJ")},
			want: headers{APIKey: "sk-ant-abc123", WorkspaceID: "wrkspc_01JwQvzr7rXLA5AGx3HKfFUJ"},
		},
		"WorkspaceIDDefault": {
			args: args{apiKey: "sk-ant-abc123", workspaceID: new("default")},
			want: headers{APIKey: "sk-ant-abc123", WorkspaceID: "default"},
		},
		"WorkspaceIDUnset": {
			args: args{apiKey: "sk-ant-abc123"},
			want: headers{APIKey: "sk-ant-abc123"},
		},
		"WorkspaceIDEmpty": {
			args: args{apiKey: "sk-ant-abc123", workspaceID: new("")},
			want: headers{APIKey: "sk-ant-abc123"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Pin the SDK's default credential lookup to the environment so a
			// developer's local profile cannot inject its own workspace header.
			t.Setenv("ANTHROPIC_API_KEY", "sk-ant-from-env")

			var got headers
			capture := option.WithMiddleware(func(req *http.Request, _ option.MiddlewareNext) (*http.Response, error) {
				got = headers{
					APIKey:      req.Header.Get("X-Api-Key"),
					WorkspaceID: req.Header.Get(workspaceIDHeader),
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader("{}")),
					Request:    req,
				}, nil
			})

			c := anthropic.NewClient(append(clientOptions(tc.args.apiKey, tc.args.workspaceID), capture)...)
			if _, err := c.Beta.Skills.Get(context.Background(), "skl_1", anthropic.BetaSkillGetParams{}); err != nil {
				t.Fatalf("Skills.Get(): %v", err)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("request headers: -want, +got:\n%s", diff)
			}
		})
	}
}

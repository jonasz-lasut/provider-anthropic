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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// exchangeServer answers the federation exchange with a minted token and
// every other request with an empty JSON body, recording what it saw. The SDK
// exchanges tokens with its own HTTP client, so only a real listener behind
// WithBaseURL can intercept both calls.
type exchangeServer struct {
	mu            sync.Mutex
	exchange      map[string]any
	authorization string
}

func (s *exchangeServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if req.URL.Path == "/v1/oauth/token" {
		raw, _ := io.ReadAll(req.Body)
		_ = json.Unmarshal(raw, &s.exchange)
		_, _ = w.Write([]byte(`{"access_token":"minted","token_type":"Bearer","expires_in":3600}`))
		return
	}
	s.authorization = req.Header.Get("Authorization")
	_, _ = w.Write([]byte("{}"))
}

func TestFederationOptions(t *testing.T) {
	type args struct {
		federation  pcv1beta1.FederationIdentity
		workspaceID *string
	}
	type want struct {
		Authorization string
		Exchange      map[string]any
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"RuleOrgServiceAccountWorkspace": {
			args: args{
				federation:  pcv1beta1.FederationIdentity{OrganizationID: "3ea445bd-41de-4b81-8def-29f091636bca", FederationRuleID: "fdrl_1", ServiceAccountID: new("svac_1")},
				workspaceID: new("wrkspc_1"),
			},
			want: want{
				Authorization: "Bearer minted",
				Exchange: map[string]any{
					"grant_type": "urn:ietf:params:oauth:grant-type:jwt-bearer", "assertion": "jwt-from-file",
					"federation_rule_id": "fdrl_1", "organization_id": "3ea445bd-41de-4b81-8def-29f091636bca", "service_account_id": "svac_1", "workspace_id": "wrkspc_1",
				},
			},
		},
		"RuleAndOrgOnly": {
			args: args{federation: pcv1beta1.FederationIdentity{OrganizationID: "3ea445bd-41de-4b81-8def-29f091636bca", FederationRuleID: "fdrl_1"}},
			want: want{
				Authorization: "Bearer minted",
				Exchange: map[string]any{
					"grant_type": "urn:ietf:params:oauth:grant-type:jwt-bearer", "assertion": "jwt-from-file",
					"federation_rule_id": "fdrl_1", "organization_id": "3ea445bd-41de-4b81-8def-29f091636bca",
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// The SDK's auth middleware yields to any static credential, so
			// leave the environment without one and point the profile lookup
			// at an empty home so a developer's local profile cannot interfere.
			t.Setenv("ANTHROPIC_API_KEY", "")
			t.Setenv("ANTHROPIC_AUTH_TOKEN", "")
			t.Setenv("HOME", t.TempDir())
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			tokenFile := filepath.Join(t.TempDir(), "token")
			if err := os.WriteFile(tokenFile, []byte("jwt-from-file\n"), 0o600); err != nil {
				t.Fatalf("write token file: %v", err)
			}
			fed := tc.args.federation
			fed.TokenFile = &tokenFile
			server := &exchangeServer{}
			srv := httptest.NewServer(server)
			defer srv.Close()

			c := anthropic.NewClient(append(federationOptions(&fed, tc.args.workspaceID), option.WithBaseURL(srv.URL))...)
			if _, err := c.Beta.Organization.Workspaces.Get(context.Background(), "wrkspc_1"); err != nil {
				t.Fatalf("Workspaces.Get(): %v", err)
			}

			got := want{Authorization: server.authorization, Exchange: server.exchange}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("federation exchange: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestFederationClient(t *testing.T) {
	const (
		org  = "3ea445bd-41de-4b81-8def-29f091636bca"
		rule = "fdrl_01AbCdEfGhIjKlMnOpQrStUv"
	)
	base := pcv1beta1.FederationIdentity{OrganizationID: org, FederationRuleID: rule}

	type args struct {
		a          pcv1beta1.FederationIdentity
		aWorkspace *string
		b          pcv1beta1.FederationIdentity
		bWorkspace *string
	}
	type want struct {
		shared bool
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"SameIdentitySharesOneClient": {
			args: args{a: base, b: base},
			want: want{shared: true},
		},
		"ExplicitDefaultTokenFileSharesOneClient": {
			args: args{a: base, b: pcv1beta1.FederationIdentity{OrganizationID: org, FederationRuleID: rule, TokenFile: new(defaultFederationTokenFile)}},
			want: want{shared: true},
		},
		"DifferentRuleGetsOwnClient": {
			args: args{a: base, b: pcv1beta1.FederationIdentity{OrganizationID: org, FederationRuleID: "fdrl_01Other"}},
			want: want{shared: false},
		},
		"DifferentServiceAccountGetsOwnClient": {
			args: args{a: base, b: pcv1beta1.FederationIdentity{OrganizationID: org, FederationRuleID: rule, ServiceAccountID: new("svac_01AbCdEfGhIjKlMnOpQrStUv")}},
			want: want{shared: false},
		},
		"DifferentWorkspaceGetsOwnClient": {
			args: args{a: base, aWorkspace: new("wrkspc_01JwQvzr7rXLA5AGx3HKfFUJ"), b: base, bWorkspace: new("default")},
			want: want{shared: false},
		},
		"DifferentTokenFileGetsOwnClient": {
			args: args{a: base, b: pcv1beta1.FederationIdentity{OrganizationID: org, FederationRuleID: rule, TokenFile: new("/var/run/secrets/other/token")}},
			want: want{shared: false},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := federationClient(&tc.args.a, tc.args.aWorkspace) == federationClient(&tc.args.b, tc.args.bWorkspace)
			if diff := cmp.Diff(tc.want.shared, got); diff != "" {
				t.Errorf("federationClient(...) shared client: -want, +got:\n%s", diff)
			}
		})
	}
}

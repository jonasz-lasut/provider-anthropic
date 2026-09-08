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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	. "github.com/jonasz-lasut/provider-anthropic/apis/organization/v1beta1"
)

var optBool = cmp.Comparer(func(a, b param.Opt[bool]) bool {
	return a.Valid() == b.Valid() && a.Value == b.Value
})

var optInt = cmp.Comparer(func(a, b param.Opt[int64]) bool {
	return a.Valid() == b.Valid() && a.Value == b.Value
})

var ignoreIssuerParamInternals = cmpopts.IgnoreUnexported(
	anthropic.BetaOrganizationFederationIssuerNewParams{},
	anthropic.BetaOrganizationFederationIssuerUpdateParams{},
	anthropic.BetaOrganizationFederationIssuerNewParamsJWKSUnion{},
	anthropic.BetaOrganizationFederationIssuerUpdateParamsJWKSUnion{},
	anthropic.BetaJWKSDiscoveryParam{},
	anthropic.BetaJWKSExplicitURLParam{},
	anthropic.BetaJWKSInlineParam{},
)

func TestFederationIssuerToAnthropicNew(t *testing.T) {
	cases := map[string]struct {
		args FederationIssuerParameters
		want anthropic.BetaOrganizationFederationIssuerNewParams
	}{
		"InlineKeys": {
			args: FederationIssuerParameters{
				Name:                  new("kind-cluster"),
				IssuerURL:             new("https://kubernetes.default.svc.cluster.local"),
				CheckJTI:              new(false),
				MaxJWTLifetimeSeconds: new(int64(7200)),
				JWKS: &FederationIssuerJWKS{Type: new("inline"), Keys: []apiextensionsv1.JSON{{Raw: []byte(`{"kty":"RSA","kid":"k1","n":"abc","e":"AQAB"}`)}}},
			},
			want: anthropic.BetaOrganizationFederationIssuerNewParams{
				Name:                  "kind-cluster",
				IssuerURL:             "https://kubernetes.default.svc.cluster.local",
				CheckJTI:              anthropic.Bool(false),
				MaxJWTLifetimeSeconds: anthropic.Int(7200),
				JWKS: anthropic.BetaOrganizationFederationIssuerNewParamsJWKSUnion{
					OfInline: &anthropic.BetaJWKSInlineParam{Keys: []map[string]any{{"kty": "RSA", "kid": "k1", "n": "abc", "e": "AQAB"}}},
				},
			},
		},
		"ExplicitURL": {
			args: FederationIssuerParameters{
				Name: new("gh"), IssuerURL: new("https://token.actions.githubusercontent.com"),
				JWKS: &FederationIssuerJWKS{Type: new("explicit_url"), URL: new("https://token.actions.githubusercontent.com/.well-known/jwks"), CACertPEM: new("PEM")},
			},
			want: anthropic.BetaOrganizationFederationIssuerNewParams{
				Name: "gh", IssuerURL: "https://token.actions.githubusercontent.com",
				JWKS: anthropic.BetaOrganizationFederationIssuerNewParamsJWKSUnion{
					OfExplicitURL: &anthropic.BetaJWKSExplicitURLParam{URL: "https://token.actions.githubusercontent.com/.well-known/jwks", CACertPEM: anthropic.String("PEM")},
				},
			},
		},
		"DiscoveryDefault": {
			args: FederationIssuerParameters{Name: new("gh"), IssuerURL: new("https://token.actions.githubusercontent.com")},
			want: anthropic.BetaOrganizationFederationIssuerNewParams{Name: "gh", IssuerURL: "https://token.actions.githubusercontent.com"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &FederationIssuer{Spec: FederationIssuerSpec{ForProvider: tc.args}}

			got := r.ToAnthropicNew()

			if diff := cmp.Diff(tc.want, got, optString, optBool, optInt, ignoreIssuerParamInternals); diff != "" {
				t.Errorf("ToAnthropicNew(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestFederationIssuerToAnthropicUpdate(t *testing.T) {
	r := &FederationIssuer{Spec: FederationIssuerSpec{ForProvider: FederationIssuerParameters{
		Name: new("renamed"), JWKSPollingDisabled: new(true),
		JWKS: &FederationIssuerJWKS{Type: new("discovery"), DiscoveryBase: new("https://example.com/oidc")},
	}}}
	want := anthropic.BetaOrganizationFederationIssuerUpdateParams{
		Name:                anthropic.String("renamed"),
		JWKSPollingDisabled: anthropic.Bool(true),
		JWKS: anthropic.BetaOrganizationFederationIssuerUpdateParamsJWKSUnion{
			OfDiscovery: &anthropic.BetaJWKSDiscoveryParam{DiscoveryBase: anthropic.String("https://example.com/oidc")},
		},
	}

	got := r.ToAnthropicUpdate()

	if diff := cmp.Diff(want, got, optString, optBool, optInt, ignoreIssuerParamInternals); diff != "" {
		t.Errorf("ToAnthropicUpdate(): -want, +got:\n%s", diff)
	}
}

func TestFederationIssuerFromAnthropicObservation(t *testing.T) {
	created := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	r := &FederationIssuer{}
	resp := anthropic.BetaFederationIssuer{
		ID: "fdis_1", Name: "kind-cluster", IssuerURL: "https://kubernetes.default.svc.cluster.local",
		CheckJTI: true, MaxJWTLifetimeSeconds: 3600, CreatedAt: created, UpdatedAt: created, ArchivedAt: created,
		JWKS:       anthropic.BetaFederationIssuerJWKSUnion{Type: "inline", Keys: []map[string]any{{"kty": "RSA", "kid": "k1"}}},
		PollStatus: anthropic.BetaFederationIssuerPollStatus{ConsecutiveFailures: 2, LastFetchedAt: created},
	}
	want := FederationIssuerObservation{
		ID: new("fdis_1"), Name: new("kind-cluster"), IssuerURL: new("https://kubernetes.default.svc.cluster.local"),
		CheckJTI: new(true), MaxJWTLifetimeSeconds: new(int64(3600)),
		JWKS:       &FederationIssuerJWKS{Type: new("inline"), Keys: []apiextensionsv1.JSON{{Raw: []byte(`{"kid":"k1","kty":"RSA"}`)}}},
		PollStatus: &FederationIssuerPollStatus{ConsecutiveFailures: new(int64(2)), LastFetchedAt: new("2026-09-08T10:00:00Z")},
		CreatedAt:  new("2026-09-08T10:00:00Z"), UpdatedAt: new("2026-09-08T10:00:00Z"),
	}

	r.FromAnthropicObservation(resp)

	if diff := cmp.Diff(want, r.Status.AtProvider); diff != "" {
		t.Errorf("FromAnthropicObservation(): -want, +got:\n%s", diff)
	}
}

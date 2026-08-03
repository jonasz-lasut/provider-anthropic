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

	. "github.com/jonasz-lasut/provider-anthropic/apis/managedagents/v1beta1"
)

func TestTunnelToAnthropicNew(t *testing.T) {
	cases := map[string]struct {
		fp   TunnelParameters
		want string // expected DisplayName.Value
	}{
		"WithDisplayName": {
			fp:   TunnelParameters{DisplayName: new("prod tunnel")},
			want: "prod tunnel",
		},
		"WithoutDisplayName": {
			fp:   TunnelParameters{},
			want: "",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &Tunnel{Spec: TunnelSpec{ForProvider: tc.fp}}
			p := r.ToAnthropicNew()
			if diff := cmp.Diff(tc.want, p.DisplayName.Value); diff != "" {
				t.Errorf("ToAnthropicNew() DisplayName.Value -want +got:\n%s", diff)
			}
		})
	}
}

func TestTunnelFromAnthropicObservation(t *testing.T) {
	created := time.Date(2026, 7, 17, 8, 0, 0, 0, time.UTC)
	resp := anthropic.BetaTunnel{
		ID:          "tnl_123",
		DisplayName: "prod tunnel",
		Domain:      "abc.tunnel.anthropic.com",
		CreatedAt:   created,
	}

	r := &Tunnel{}
	r.FromAnthropicObservation(resp)

	want := TunnelObservation{
		ID:          new("tnl_123"),
		DisplayName: new("prod tunnel"),
		Domain:      new("abc.tunnel.anthropic.com"),
		CreatedAt:   new(created.Format(time.RFC3339)),
	}
	if diff := cmp.Diff(want, r.Status.AtProvider); diff != "" {
		t.Errorf("FromAnthropicObservation() -want +got:\n%s", diff)
	}
}

func TestTunnelToConnectionDetails(t *testing.T) {
	cases := map[string]struct {
		cctx TunnelConversionContext
		want map[string][]byte
	}{
		"DomainTokenAndID": {
			cctx: TunnelConversionContext{Domain: "abc.tunnel.anthropic.com", TunnelToken: "tok_secret", TokenID: "ttok_1"},
			want: map[string][]byte{
				"domain":      []byte("abc.tunnel.anthropic.com"),
				"tunnelToken": []byte("tok_secret"),
				"tokenId":     []byte("ttok_1"),
			},
		},
		"DomainOnly": {
			cctx: TunnelConversionContext{Domain: "abc.tunnel.anthropic.com"},
			want: map[string][]byte{
				"domain": []byte("abc.tunnel.anthropic.com"),
			},
		},
		"Empty": {
			cctx: TunnelConversionContext{},
			want: map[string][]byte{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.cctx.ToConnectionDetails()
			if diff := cmp.Diff(tc.want, map[string][]byte(got)); diff != "" {
				t.Errorf("ToConnectionDetails() -want +got:\n%s", diff)
			}
		})
	}
}

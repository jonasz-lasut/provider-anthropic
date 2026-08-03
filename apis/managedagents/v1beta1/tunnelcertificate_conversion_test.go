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

func TestTunnelCertificateToAnthropicNew(t *testing.T) {
	cases := map[string]struct {
		ctx  *TunnelCertificateConversionContext
		want string // expected CaCertificatePem
	}{
		"WithPem": {
			ctx:  &TunnelCertificateConversionContext{CaCertificatePem: "-----BEGIN CERTIFICATE-----\nabc\n-----END CERTIFICATE-----"},
			want: "-----BEGIN CERTIFICATE-----\nabc\n-----END CERTIFICATE-----",
		},
		"NilContext": {
			ctx:  nil,
			want: "",
		},
		"EmptyPem": {
			ctx:  &TunnelCertificateConversionContext{},
			want: "",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &TunnelCertificate{}
			p := r.ToAnthropicNew(tc.ctx)
			if diff := cmp.Diff(tc.want, p.CaCertificatePem); diff != "" {
				t.Errorf("ToAnthropicNew() CaCertificatePem -want +got:\n%s", diff)
			}
		})
	}
}

func TestTunnelCertificateFromAnthropicObservation(t *testing.T) {
	created := time.Date(2026, 7, 17, 8, 0, 0, 0, time.UTC)
	expires := time.Date(2027, 7, 17, 8, 0, 0, 0, time.UTC)
	resp := anthropic.BetaTunnelCertificate{
		ID:          "tcrt_123",
		TunnelID:    "tnl_1",
		Fingerprint: "abc123",
		ExpiresAt:   expires,
		CreatedAt:   created,
	}

	r := &TunnelCertificate{}
	r.FromAnthropicObservation(resp)

	want := TunnelCertificateObservation{
		ID:          new("tcrt_123"),
		TunnelID:    new("tnl_1"),
		Fingerprint: new("abc123"),
		ExpiresAt:   new(expires.Format(time.RFC3339)),
		CreatedAt:   new(created.Format(time.RFC3339)),
	}
	if diff := cmp.Diff(want, r.Status.AtProvider); diff != "" {
		t.Errorf("FromAnthropicObservation() -want +got:\n%s", diff)
	}
}

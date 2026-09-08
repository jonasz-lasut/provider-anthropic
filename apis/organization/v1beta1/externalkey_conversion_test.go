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

var ignoreExternalKeyParamInternals = cmpopts.IgnoreUnexported(
	anthropic.BetaOrganizationExternalKeyNewParams{},
	anthropic.BetaOrganizationExternalKeyUpdateParams{},
	anthropic.BetaOrganizationExternalKeyNewParamsProviderConfigUnion{},
	anthropic.BetaOrganizationExternalKeyUpdateParamsProviderConfigUnion{},
	anthropic.BetaAWSExternalKeyConfigParam{},
	anthropic.BetaGCPExternalKeyConfigParam{},
	anthropic.BetaAzureExternalKeyConfigParam{},
)

func TestExternalKeyToAnthropicNew(t *testing.T) {
	cases := map[string]struct {
		args ExternalKeyParameters
		want anthropic.BetaOrganizationExternalKeyNewParams
	}{
		"AWS": {
			args: ExternalKeyParameters{
				DisplayName:    new("prod-cmek"),
				Geo:            new("us"),
				ProviderConfig: &ExternalKeyProviderConfig{Type: new("aws"), KMSARN: new("arn:aws:kms:us-east-1:123456789012:key/abc"), Region: new("us-east-1")},
			},
			want: anthropic.BetaOrganizationExternalKeyNewParams{
				DisplayName: anthropic.String("prod-cmek"),
				Geo:         anthropic.BetaOrganizationExternalKeyNewParamsGeoUs,
				ProviderConfig: anthropic.BetaOrganizationExternalKeyNewParamsProviderConfigUnion{
					OfAWS: &anthropic.BetaAWSExternalKeyConfigParam{KMSARN: "arn:aws:kms:us-east-1:123456789012:key/abc", Region: anthropic.String("us-east-1")},
				},
			},
		},
		"GCP": {
			args: ExternalKeyParameters{ProviderConfig: &ExternalKeyProviderConfig{Type: new("gcp"), KeyName: new("projects/p/locations/us/keyRings/r/cryptoKeys/k")}},
			want: anthropic.BetaOrganizationExternalKeyNewParams{
				ProviderConfig: anthropic.BetaOrganizationExternalKeyNewParamsProviderConfigUnion{
					OfGCP: &anthropic.BetaGCPExternalKeyConfigParam{KeyName: "projects/p/locations/us/keyRings/r/cryptoKeys/k"},
				},
			},
		},
		"Azure": {
			args: ExternalKeyParameters{ProviderConfig: &ExternalKeyProviderConfig{Type: new("azure"), KeyName: new("k"), TenantID: new("tenant"), VaultURI: new("https://v.vault.azure.net"), ClientID: new("client")}},
			want: anthropic.BetaOrganizationExternalKeyNewParams{
				ProviderConfig: anthropic.BetaOrganizationExternalKeyNewParamsProviderConfigUnion{
					OfAzure: &anthropic.BetaAzureExternalKeyConfigParam{KeyName: "k", TenantID: "tenant", VaultURI: "https://v.vault.azure.net", ClientID: anthropic.String("client")},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &ExternalKey{Spec: ExternalKeySpec{ForProvider: tc.args}}

			got := r.ToAnthropicNew()

			if diff := cmp.Diff(tc.want, got, optString, ignoreExternalKeyParamInternals); diff != "" {
				t.Errorf("ToAnthropicNew(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestExternalKeyToAnthropicUpdate(t *testing.T) {
	r := &ExternalKey{Spec: ExternalKeySpec{ForProvider: ExternalKeyParameters{
		DisplayName:    new("renamed"),
		ProviderConfig: &ExternalKeyProviderConfig{Type: new("aws"), KMSARN: new("arn:aws:kms:us-east-1:123456789012:key/abc")},
	}}}
	want := anthropic.BetaOrganizationExternalKeyUpdateParams{
		DisplayName: anthropic.String("renamed"),
		ProviderConfig: anthropic.BetaOrganizationExternalKeyUpdateParamsProviderConfigUnion{
			OfAWS: &anthropic.BetaAWSExternalKeyConfigParam{KMSARN: "arn:aws:kms:us-east-1:123456789012:key/abc"},
		},
	}

	got := r.ToAnthropicUpdate()

	if diff := cmp.Diff(want, got, optString, ignoreExternalKeyParamInternals); diff != "" {
		t.Errorf("ToAnthropicUpdate(): -want, +got:\n%s", diff)
	}
}

func TestExternalKeyFromAnthropicObservation(t *testing.T) {
	created := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	r := &ExternalKey{}
	resp := anthropic.BetaExternalKey{
		ID: "ekey_1", DisplayName: "prod-cmek", Geo: "us", CreatedAt: created, UpdatedAt: created,
		ProviderConfig: anthropic.BetaExternalKeyProviderConfigUnion{Type: "aws", KMSARN: "arn:aws:kms:us-east-1:123456789012:key/abc", Region: "us-east-1"},
		Attachment:     anthropic.BetaExternalKeyAttachmentUnion{Type: "unattached"},
	}
	want := ExternalKeyObservation{
		ID: new("ekey_1"), DisplayName: new("prod-cmek"), Geo: new("us"), Attachment: new("unattached"),
		ProviderConfig: &ExternalKeyProviderConfig{Type: new("aws"), KMSARN: new("arn:aws:kms:us-east-1:123456789012:key/abc"), Region: new("us-east-1")},
		CreatedAt:      new("2026-09-08T10:00:00Z"), UpdatedAt: new("2026-09-08T10:00:00Z"),
	}

	r.FromAnthropicObservation(resp)

	if diff := cmp.Diff(want, r.Status.AtProvider); diff != "" {
		t.Errorf("FromAnthropicObservation(): -want, +got:\n%s", diff)
	}
}

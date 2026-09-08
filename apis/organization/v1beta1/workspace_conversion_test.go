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
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	. "github.com/jonasz-lasut/provider-anthropic/apis/organization/v1beta1"
)

// optString compares param.Opt[string] by validity and value; the SDK keeps
// the omitted-or-valid state in unexported fields.
var optString = cmp.Comparer(func(a, b param.Opt[string]) bool {
	return a.Valid() == b.Valid() && a.Value == b.Value
})

var ignoreParamInternals = cmpopts.IgnoreUnexported(
	anthropic.BetaOrganizationWorkspaceNewParams{},
	anthropic.BetaOrganizationWorkspaceUpdateParams{},
	anthropic.BetaDataResidencyCreateConfigParam{},
	anthropic.BetaDataResidencyCreateConfigAllowedInferenceGeosUnionParam{},
	anthropic.BetaDataResidencyUpdateConfigParam{},
	anthropic.BetaDataResidencyUpdateConfigAllowedInferenceGeosUnionParam{},
)

func TestWorkspaceToAnthropicNew(t *testing.T) {
	cases := map[string]struct {
		args WorkspaceParameters
		want anthropic.BetaOrganizationWorkspaceNewParams
	}{
		"AllFieldsWithGeoList": {
			args: WorkspaceParameters{
				Name:          new("prod"),
				DisplayColor:  new("#6C5BB9"),
				ExternalKeyID: new("ekey_1"),
				Tags:          map[string]string{"team": "platform"},
				DataResidency: &WorkspaceDataResidency{
					WorkspaceGeo:         new("us"),
					AllowedInferenceGeos: []string{"us", "global"},
					DefaultInferenceGeo:  new("us"),
				},
			},
			want: anthropic.BetaOrganizationWorkspaceNewParams{
				Name:          "prod",
				DisplayColor:  anthropic.String("#6C5BB9"),
				ExternalKeyID: anthropic.String("ekey_1"),
				Tags:          map[string]string{"team": "platform"},
				DataResidency: anthropic.BetaDataResidencyCreateConfigParam{
					WorkspaceGeo:        anthropic.BetaDataResidencyCreateConfigWorkspaceGeoUs,
					DefaultInferenceGeo: anthropic.BetaDataResidencyCreateConfigDefaultInferenceGeoUs,
					AllowedInferenceGeos: anthropic.BetaDataResidencyCreateConfigAllowedInferenceGeosUnionParam{
						OfGeos: []anthropic.BetaAllowedInferenceGeo{anthropic.BetaAllowedInferenceGeoUs, anthropic.BetaAllowedInferenceGeoGlobal},
					},
				},
			},
		},
		"UnrestrictedGeos": {
			args: WorkspaceParameters{
				Name:          new("prod"),
				DataResidency: &WorkspaceDataResidency{AllowedInferenceGeos: []string{"unrestricted"}},
			},
			want: anthropic.BetaOrganizationWorkspaceNewParams{
				Name: "prod",
				DataResidency: anthropic.BetaDataResidencyCreateConfigParam{
					AllowedInferenceGeos: anthropic.BetaDataResidencyCreateConfigAllowedInferenceGeosUnionParam{
						OfUnrestricted: constant.ValueOf[constant.Unrestricted](),
					},
				},
			},
		},
		"NameOnly": {
			args: WorkspaceParameters{Name: new("prod")},
			want: anthropic.BetaOrganizationWorkspaceNewParams{Name: "prod"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &Workspace{Spec: WorkspaceSpec{ForProvider: tc.args}}

			got := r.ToAnthropicNew()

			if diff := cmp.Diff(tc.want, got, optString, ignoreParamInternals); diff != "" {
				t.Errorf("ToAnthropicNew(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestWorkspaceToAnthropicUpdate(t *testing.T) {
	cases := map[string]struct {
		args struct {
			params   WorkspaceParameters
			observed WorkspaceObservation
		}
		want anthropic.BetaOrganizationWorkspaceUpdateParams
	}{
		"MutableFieldsWithoutWorkspaceGeo": {
			args: struct {
				params   WorkspaceParameters
				observed WorkspaceObservation
			}{
				params: WorkspaceParameters{
					Name:         new("renamed"),
					DisplayColor: new("#000000"),
					Tags:         map[string]string{"team": "ml"},
					DataResidency: &WorkspaceDataResidency{
						WorkspaceGeo:         new("us"),
						AllowedInferenceGeos: []string{"unrestricted"},
						DefaultInferenceGeo:  new("global"),
					},
				},
			},
			want: anthropic.BetaOrganizationWorkspaceUpdateParams{
				Name:         anthropic.String("renamed"),
				DisplayColor: anthropic.String("#000000"),
				Tags:         map[string]string{"team": "ml"},
				DataResidency: anthropic.BetaDataResidencyUpdateConfigParam{
					DefaultInferenceGeo: anthropic.BetaDataResidencyUpdateConfigDefaultInferenceGeoGlobal,
					AllowedInferenceGeos: anthropic.BetaDataResidencyUpdateConfigAllowedInferenceGeosUnionParam{
						OfUnrestricted: constant.ValueOf[constant.Unrestricted](),
					},
				},
			},
		},
		"ExternalKeySentWhenNotYetAttached": {
			args: struct {
				params   WorkspaceParameters
				observed WorkspaceObservation
			}{
				params: WorkspaceParameters{ExternalKeyID: new("ekey_1")},
			},
			want: anthropic.BetaOrganizationWorkspaceUpdateParams{ExternalKeyID: anthropic.String("ekey_1")},
		},
		"ExternalKeyOmittedWhenAlreadyAttached": {
			args: struct {
				params   WorkspaceParameters
				observed WorkspaceObservation
			}{
				params:   WorkspaceParameters{ExternalKeyID: new("ekey_1")},
				observed: WorkspaceObservation{ExternalKeyID: new("ekey_1")},
			},
			want: anthropic.BetaOrganizationWorkspaceUpdateParams{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &Workspace{
				Spec:   WorkspaceSpec{ForProvider: tc.args.params},
				Status: WorkspaceStatus{AtProvider: tc.args.observed},
			}

			got := r.ToAnthropicUpdate()

			if diff := cmp.Diff(tc.want, got, optString, ignoreParamInternals); diff != "" {
				t.Errorf("ToAnthropicUpdate(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestWorkspaceFromAnthropicObservation(t *testing.T) {
	created := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)

	cases := map[string]struct {
		args anthropic.BetaWorkspace
		want WorkspaceObservation
	}{
		"GeoListAndExternalKey": {
			args: anthropic.BetaWorkspace{
				ID:            "wrkspc_1",
				Name:          "prod",
				DisplayColor:  "#6C5BB9",
				ExternalKeyID: "ekey_1",
				CompartmentID: "cmpt_1",
				Tags:          map[string]string{"team": "platform"},
				CreatedAt:     created,
				ArchivedAt:    created,
				DataResidency: anthropic.BetaDataResidency{
					WorkspaceGeo:         "us",
					DefaultInferenceGeo:  "us",
					AllowedInferenceGeos: anthropic.BetaDataResidencyAllowedInferenceGeosUnion{OfGeos: []string{"us"}},
				},
			},
			want: WorkspaceObservation{
				ID:            new("wrkspc_1"),
				Name:          new("prod"),
				DisplayColor:  new("#6C5BB9"),
				ExternalKeyID: new("ekey_1"),
				CompartmentID: new("cmpt_1"),
				Tags:          map[string]string{"team": "platform"},
				CreatedAt:     new("2026-09-08T10:00:00Z"),
				DataResidency: &WorkspaceDataResidency{
					WorkspaceGeo:         new("us"),
					AllowedInferenceGeos: []string{"us"},
					DefaultInferenceGeo:  new("us"),
				},
			},
		},
		"UnrestrictedAndNoExternalKey": {
			args: anthropic.BetaWorkspace{
				ID:            "wrkspc_2",
				Name:          "dev",
				CompartmentID: "cmpt_1",
				Tags:          map[string]string{},
				CreatedAt:     created,
				DataResidency: anthropic.BetaDataResidency{
					WorkspaceGeo:         "us",
					DefaultInferenceGeo:  "global",
					AllowedInferenceGeos: anthropic.BetaDataResidencyAllowedInferenceGeosUnion{OfUnrestricted: constant.ValueOf[constant.Unrestricted]()},
				},
			},
			want: WorkspaceObservation{
				ID:            new("wrkspc_2"),
				Name:          new("dev"),
				DisplayColor:  new(""),
				CompartmentID: new("cmpt_1"),
				Tags:          map[string]string{},
				CreatedAt:     new("2026-09-08T10:00:00Z"),
				DataResidency: &WorkspaceDataResidency{
					WorkspaceGeo:         new("us"),
					AllowedInferenceGeos: []string{"unrestricted"},
					DefaultInferenceGeo:  new("global"),
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &Workspace{}

			r.FromAnthropicObservation(tc.args)

			if diff := cmp.Diff(tc.want, r.Status.AtProvider); diff != "" {
				t.Errorf("FromAnthropicObservation(): -want, +got:\n%s", diff)
			}
		})
	}
}

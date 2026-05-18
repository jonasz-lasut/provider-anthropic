/*
Copyright 2025 The provider-anthropic-platform Authors.

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

package predicates

import (
	"testing"

	betav1alpha1 "github.com/jonasz-lasut/provider-anthropic-platform/apis/beta/v1alpha1"
	"k8s.io/utils/ptr"
)

// fakeResource is a stand-in for any Anthropic SDK response item.
type fakeResource struct {
	Name     string            `json:"name"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Config   map[string]any    `json:"config,omitempty"`
}

func TestClientSideFilter(t *testing.T) {
	tests := []struct {
		name     string
		pred     *betav1alpha1.Predicates
		item     fakeResource
		wantIncl bool
		wantErr  bool
	}{
		{
			name:     "nil predicates passes all",
			pred:     nil,
			item:     fakeResource{Name: "foo"},
			wantIncl: true,
		},
		{
			name:     "empty predicates passes all",
			pred:     &betav1alpha1.Predicates{},
			item:     fakeResource{Name: "foo"},
			wantIncl: true,
		},
		// MetadataMatch cases
		{
			name: "metadataMatch single pair passes",
			pred: &betav1alpha1.Predicates{
				MetadataMatch: map[string]string{"managed-by": "crossplane"},
			},
			item:     fakeResource{Metadata: map[string]string{"managed-by": "crossplane"}},
			wantIncl: true,
		},
		{
			name: "metadataMatch single pair excluded",
			pred: &betav1alpha1.Predicates{
				MetadataMatch: map[string]string{"managed-by": "crossplane"},
			},
			item:     fakeResource{Metadata: map[string]string{"managed-by": "manual"}},
			wantIncl: false,
		},
		{
			name: "metadataMatch extra keys on resource allowed",
			pred: &betav1alpha1.Predicates{
				MetadataMatch: map[string]string{"managed-by": "crossplane"},
			},
			item: fakeResource{Metadata: map[string]string{
				"managed-by": "crossplane",
				"env":        "prod",
			}},
			wantIncl: true,
		},
		{
			name: "metadataMatch no metadata on resource excluded",
			pred: &betav1alpha1.Predicates{
				MetadataMatch: map[string]string{"managed-by": "crossplane"},
			},
			item:     fakeResource{Name: "bar"},
			wantIncl: false,
		},
		// CELFilter cases
		{
			name:     "celFilter literal true passes",
			pred:     &betav1alpha1.Predicates{CELFilter: ptr.To(`true`)},
			item:     fakeResource{Name: "foo"},
			wantIncl: true,
		},
		{
			name:     "celFilter literal false excluded",
			pred:     &betav1alpha1.Predicates{CELFilter: ptr.To(`false`)},
			item:     fakeResource{Name: "foo"},
			wantIncl: false,
		},
		{
			name:     "celFilter field match passes",
			pred:     &betav1alpha1.Predicates{CELFilter: ptr.To(`atProvider.name == "target"`)},
			item:     fakeResource{Name: "target"},
			wantIncl: true,
		},
		{
			name:     "celFilter field match excluded",
			pred:     &betav1alpha1.Predicates{CELFilter: ptr.To(`atProvider.name == "target"`)},
			item:     fakeResource{Name: "other"},
			wantIncl: false,
		},
		{
			name: "celFilter nested config field",
			pred: &betav1alpha1.Predicates{
				CELFilter: ptr.To(`atProvider.config.networking == "unrestricted"`),
			},
			item: fakeResource{
				Config: map[string]any{"networking": "unrestricted"},
			},
			wantIncl: true,
		},
		{
			name:    "celFilter invalid expression yields error",
			pred:    &betav1alpha1.Predicates{CELFilter: ptr.To(`not valid !!!`)},
			item:    fakeResource{Name: "foo"},
			wantErr: true,
		},
		// Combined MetadataMatch + CELFilter (AND semantics)
		{
			name: "both predicates pass",
			pred: &betav1alpha1.Predicates{
				MetadataMatch: map[string]string{"env": "prod"},
				CELFilter:     ptr.To(`atProvider.name == "svc"`),
			},
			item: fakeResource{
				Name:     "svc",
				Metadata: map[string]string{"env": "prod"},
			},
			wantIncl: true,
		},
		{
			name: "metadataMatch fails short-circuits CEL",
			pred: &betav1alpha1.Predicates{
				MetadataMatch: map[string]string{"env": "prod"},
				CELFilter:     ptr.To(`true`),
			},
			item:     fakeResource{Metadata: map[string]string{"env": "dev"}},
			wantIncl: false,
		},
		{
			name: "metadataMatch passes but CEL excludes",
			pred: &betav1alpha1.Predicates{
				MetadataMatch: map[string]string{"env": "prod"},
				CELFilter:     ptr.To(`atProvider.name == "svc"`),
			},
			item: fakeResource{
				Name:     "other",
				Metadata: map[string]string{"env": "prod"},
			},
			wantIncl: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ClientSideFilter(tt.pred, tt.item)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantIncl {
				t.Errorf("ClientSideFilter() = %v, want %v", got, tt.wantIncl)
			}
		})
	}
}

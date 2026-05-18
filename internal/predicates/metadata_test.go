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

	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
)

func TestMatchesMetadata(t *testing.T) {
	tests := []struct {
		name  string
		obj   map[string]any
		match map[string]string
		want  bool
	}{
		{
			name:  "nil match passes all (matchLabels semantics)",
			obj:   map[string]any{"metadata": map[string]any{"key": "val"}},
			match: nil,
			want:  true,
		},
		{
			name:  "empty match passes all (matchLabels semantics)",
			obj:   map[string]any{"metadata": map[string]any{"key": "val"}},
			match: map[string]string{},
			want:  true,
		},
		{
			name:  "single pair matches",
			obj:   map[string]any{"metadata": map[string]any{"managed-by": "crossplane"}},
			match: map[string]string{"managed-by": "crossplane"},
			want:  true,
		},
		{
			name:  "single pair value differs",
			obj:   map[string]any{"metadata": map[string]any{"managed-by": "manual"}},
			match: map[string]string{"managed-by": "crossplane"},
			want:  false,
		},
		{
			name: "all pairs match",
			obj: map[string]any{
				"metadata": map[string]any{"managed-by": "crossplane", "env": "prod"},
			},
			match: map[string]string{"managed-by": "crossplane", "env": "prod"},
			want:  true,
		},
		{
			name: "extra metadata keys allowed (superset)",
			obj: map[string]any{
				"metadata": map[string]any{
					"managed-by":    "crossplane",
					"env":           "prod",
					"extra-key":     "ignored",
					"crossplane-id": "abc123",
				},
			},
			match: map[string]string{"managed-by": "crossplane"},
			want:  true,
		},
		{
			name: "one of multiple pairs value differs",
			obj: map[string]any{
				"metadata": map[string]any{"managed-by": "crossplane", "env": "dev"},
			},
			match: map[string]string{"managed-by": "crossplane", "env": "prod"},
			want:  false,
		},
		{
			name:  "required key absent in metadata",
			obj:   map[string]any{"metadata": map[string]any{"other-key": "val"}},
			match: map[string]string{"managed-by": "crossplane"},
			want:  false,
		},
		{
			name:  "missing metadata field",
			obj:   map[string]any{"name": "foo"},
			match: map[string]string{"managed-by": "crossplane"},
			want:  false,
		},
		{
			name:  "null metadata field",
			obj:   map[string]any{"metadata": nil},
			match: map[string]string{"managed-by": "crossplane"},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paved := fieldpath.Pave(tt.obj)
			got, err := MatchesMetadata(paved, tt.match)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("MatchesMetadata() = %v, want %v", got, tt.want)
			}
		})
	}
}

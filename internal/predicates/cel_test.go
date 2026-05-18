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
	"strings"
	"testing"
)

func TestPredicateDeriveFromCelQuery(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		obj       map[string]any
		wantIncl  bool
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "literal true always includes",
			query:    `true`,
			obj:      map[string]any{},
			wantIncl: true,
		},
		{
			name:     "literal false always excludes",
			query:    `false`,
			obj:      map[string]any{},
			wantIncl: false,
		},
		{
			name:      "non-bool return type yields error",
			query:     `42`,
			obj:       map[string]any{},
			wantErr:   true,
			errSubstr: errCelQueryReturnTypeNotBool,
		},
		{
			name:      "invalid CEL syntax yields error",
			query:     `this is not @valid!!!`,
			obj:       map[string]any{},
			wantErr:   true,
			errSubstr: errCelQueryFailedToCompile,
		},
		{
			name:     "map key equality match",
			query:    `atProvider["name"] == "my-agent"`,
			obj:      map[string]any{"name": "my-agent"},
			wantIncl: true,
		},
		{
			name:     "map key equality no match",
			query:    `atProvider["name"] == "my-agent"`,
			obj:      map[string]any{"name": "other"},
			wantIncl: false,
		},
		{
			name:  "nested map key equality match",
			query: `atProvider["config"]["networking"] == "unrestricted"`,
			obj: map[string]any{
				"config": map[string]any{"networking": "unrestricted"},
			},
			wantIncl: true,
		},
		{
			name:  "nested map key equality no match",
			query: `atProvider["config"]["networking"] == "unrestricted"`,
			obj: map[string]any{
				"config": map[string]any{"networking": "private"},
			},
			wantIncl: false,
		},
		{
			name:  "metadata managed-by filter",
			query: `atProvider["metadata"]["managed-by"] == "crossplane"`,
			obj: map[string]any{
				"metadata": map[string]any{"managed-by": "crossplane"},
			},
			wantIncl: true,
		},
		{
			name:  "metadata managed-by filter no match",
			query: `atProvider["metadata"]["managed-by"] == "crossplane"`,
			obj: map[string]any{
				"metadata": map[string]any{"managed-by": "manual"},
			},
			wantIncl: false,
		},
		{
			name:     "key presence using in operator",
			query:    `"model" in atProvider`,
			obj:      map[string]any{"model": "claude-opus-4-7"},
			wantIncl: true,
		},
		{
			name:     "key absence using in operator",
			query:    `"model" in atProvider`,
			obj:      map[string]any{"name": "foo"},
			wantIncl: false,
		},
		{
			name:     "boolean conjunction all match",
			query:    `atProvider["env"] == "prod" && atProvider["tier"] == "premium"`,
			obj:      map[string]any{"env": "prod", "tier": "premium"},
			wantIncl: true,
		},
		{
			name:     "boolean conjunction partial miss",
			query:    `atProvider["env"] == "prod" && atProvider["tier"] == "premium"`,
			obj:      map[string]any{"env": "prod", "tier": "free"},
			wantIncl: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PredicateDeriveFromCelQuery(tt.query, tt.obj)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantIncl {
				t.Errorf("included = %v, want %v", got, tt.wantIncl)
			}
		})
	}
}

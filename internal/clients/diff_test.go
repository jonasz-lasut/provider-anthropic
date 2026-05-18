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

package clients

import (
	"testing"
)

func TestIsSubsetEqual(t *testing.T) {
	tests := []struct {
		name     string
		desired  map[string]any
		observed map[string]any
		want     bool
	}{
		{
			name:    "identical flat maps",
			desired: map[string]any{"name": "foo", "type": "bar"},
			observed: map[string]any{"name": "foo", "type": "bar"},
			want:    true,
		},
		{
			name:    "observed has extra keys - should still be equal",
			desired: map[string]any{"name": "foo"},
			observed: map[string]any{"name": "foo", "id": "123", "createdAt": "..."},
			want:    true,
		},
		{
			name:    "desired key absent in observed - skipped",
			desired: map[string]any{"secretRef": map[string]any{"name": "s", "key": "k"}},
			observed: map[string]any{"name": "foo"},
			want:    true,
		},
		{
			name:    "string vs id-map model comparison - equal",
			desired: map[string]any{"model": "claude-opus-4-7"},
			observed: map[string]any{"model": map[string]any{"id": "claude-opus-4-7", "type": "model"}},
			want:    true,
		},
		{
			name:    "string vs id-map model comparison - not equal",
			desired: map[string]any{"model": "claude-haiku-4-5"},
			observed: map[string]any{"model": map[string]any{"id": "claude-opus-4-7", "type": "model"}},
			want:    false,
		},
		{
			// The actual bug: API returns name:"" on tool objects, ForProvider has no name.
			// slice elements are maps, so subset semantics apply element-by-element.
			name: "slice of maps - observed element has extra empty-string field",
			desired: map[string]any{
				"tools": []any{
					map[string]any{"type": "agent_toolset_20260401"},
				},
			},
			observed: map[string]any{
				"tools": []any{
					map[string]any{"type": "agent_toolset_20260401", "name": ""},
				},
			},
			want: true,
		},
		{
			name: "slice of maps - element field value differs",
			desired: map[string]any{
				"tools": []any{
					map[string]any{"type": "agent_toolset_20260401"},
				},
			},
			observed: map[string]any{
				"tools": []any{
					map[string]any{"type": "other_type", "name": ""},
				},
			},
			want: false,
		},
		{
			name: "slice length mismatch",
			desired: map[string]any{
				"tools": []any{
					map[string]any{"type": "a"},
					map[string]any{"type": "b"},
				},
			},
			observed: map[string]any{
				"tools": []any{
					map[string]any{"type": "a"},
				},
			},
			want: false,
		},
		{
			name: "mcpServers - observed has extra fields per element",
			desired: map[string]any{
				"mcpServers": []any{
					map[string]any{"name": "my-server", "url": "https://example.com"},
				},
			},
			observed: map[string]any{
				"mcpServers": []any{
					map[string]any{"name": "my-server", "url": "https://example.com", "status": "active"},
				},
			},
			want: true,
		},
		{
			name: "metadata subset comparison",
			desired: map[string]any{
				"metadata": map[string]any{"example": "true"},
			},
			observed: map[string]any{
				"metadata": map[string]any{"example": "true", "crossplane-name": "foo"},
			},
			want: true,
		},
		{
			name: "metadata value drift",
			desired: map[string]any{
				"metadata": map[string]any{"example": "true"},
			},
			observed: map[string]any{
				"metadata": map[string]any{"example": "false"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSubsetEqual(tt.desired, tt.observed)
			if got != tt.want {
				t.Errorf("IsSubsetEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

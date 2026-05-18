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
	"encoding/json"
	"reflect"
	"strings"
)

// PopulateAtProvider translates an SDK response struct (snake_case JSON keys)
// into a Crossplane AtProvider observation struct (camelCase JSON keys).
// It marshals resp to JSON, converts all snake_case map keys to camelCase
// recursively, deletes any top-level keys listed in excludeKeys (e.g.
// "archived_at" whose zero-value is ambiguous, or "auth" for write-only
// credential fields), then unmarshals the result into out.
//
// Fields in the SDK response that have no corresponding field in the AtProvider
// struct are silently ignored (standard json.Unmarshal behaviour).
// Fields that have incompatible types are also silently ignored, so callers
// should assign those fields manually after calling this function.
func PopulateAtProvider[R any, O any](resp R, out *O, excludeKeys ...string) error {
	raw, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}

	for _, k := range excludeKeys {
		delete(m, k)
	}

	camel := snakeToCamelMap(m)

	finalRaw, err := json.Marshal(camel)
	if err != nil {
		return err
	}

	return json.Unmarshal(finalRaw, out)
}

// IsSubsetEqual returns true when every key present in desired also has an
// equal value in observed, or is absent from observed entirely.
//
// Absent-from-observed keys are skipped rather than treated as drift. This
// handles ForProvider-only fields (SecretRefs, DeletionPolicy, cross-resource
// Refs/Selectors) that have no API counterpart and therefore never appear in
// the AtProvider JSON.
//
// Nested maps are compared recursively. Slices and scalars are compared with
// reflect.DeepEqual.
//
// Special case: when desired[k] is a plain string and observed[k] is a map
// containing an "id" key, the string is compared against that id. This
// handles the Anthropic SDK pattern where ForProvider stores a plain resource
// ID string (e.g. model: "claude-opus-4-7") but the API returns a typed
// object (e.g. model: {id: "claude-opus-4-7", type: "model"}).
func IsSubsetEqual(desired, observed map[string]any) bool {
	for k, dv := range desired {
		ov, exists := observed[k]
		if !exists {
			continue
		}

		if dvMap, ok := dv.(map[string]any); ok {
			ovMap, ok := ov.(map[string]any)
			if !ok {
				return false
			}
			if !IsSubsetEqual(dvMap, ovMap) {
				return false
			}
			continue
		}

		if dvStr, ok := dv.(string); ok {
			if ovMap, ok := ov.(map[string]any); ok {
				ovID, _ := ovMap["id"].(string)
				if dvStr == ovID {
					continue
				}
				return false
			}
		}

		if dvSlice, ok := dv.([]any); ok {
			ovSlice, ok := ov.([]any)
			if !ok || len(dvSlice) != len(ovSlice) {
				return false
			}
			for i, dvElem := range dvSlice {
				ovElem := ovSlice[i]
				if dvElemMap, ok := dvElem.(map[string]any); ok {
					ovElemMap, ok := ovElem.(map[string]any)
					if !ok {
						return false
					}
					if !IsSubsetEqual(dvElemMap, ovElemMap) {
						return false
					}
				} else if !reflect.DeepEqual(dvElem, ovElem) {
					return false
				}
			}
			continue
		}

		if !reflect.DeepEqual(dv, ov) {
			return false
		}
	}
	return true
}

// snakeToCamelMap converts all snake_case keys in m to camelCase, recursively
// processing nested maps and slices of maps.
func snakeToCamelMap(m map[string]any) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		switch val := v.(type) {
		case map[string]any:
			result[snakeToCamelCase(k)] = snakeToCamelMap(val)
		case []any:
			result[snakeToCamelCase(k)] = convertSlice(val)
		default:
			result[snakeToCamelCase(k)] = v
		}
	}
	return result
}

func convertSlice(s []any) []any {
	result := make([]any, len(s))
	for i, v := range s {
		if m, ok := v.(map[string]any); ok {
			result[i] = snakeToCamelMap(m)
		} else {
			result[i] = v
		}
	}
	return result
}

// snakeToCamelCase converts a single snake_case identifier to camelCase.
// Single-word identifiers are returned unchanged. Examples:
//
//	"created_at"     → "createdAt"
//	"mcp_servers"    → "mcpServers"
//	"vault_id"       → "vaultId"
//	"content_sha256" → "contentSha256"
func snakeToCamelCase(s string) string {
	parts := strings.Split(s, "_")
	if len(parts) == 1 {
		return s
	}
	var b strings.Builder
	b.WriteString(parts[0])
	for _, p := range parts[1:] {
		if len(p) > 0 {
			b.WriteString(strings.ToUpper(p[:1]))
			b.WriteString(p[1:])
		}
	}
	return b.String()
}

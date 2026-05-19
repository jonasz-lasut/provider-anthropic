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

package clients

import (
	"reflect"
)

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

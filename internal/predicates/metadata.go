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

package predicates

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
)

// MatchesMetadata returns true if the Paved object's "metadata" map is a
// superset of match — the same semantics as Kubernetes matchLabels: every
// key-value pair in match must be present in the resource's metadata, but
// extra metadata keys are allowed. A nil or empty match passes all resources.
func MatchesMetadata(paved *fieldpath.Paved, match map[string]string) (bool, error) {
	if len(match) == 0 {
		return true, nil
	}
	meta, err := paved.GetStringObject("metadata")
	if err != nil {
		// Absent, null, or non-string-map metadata cannot satisfy any match.
		return false, nil
	}
	for k, v := range match {
		if meta[k] != v {
			return false, nil
		}
	}
	return true, nil
}

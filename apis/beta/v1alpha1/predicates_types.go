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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Predicates is the generic filter applied by every Observed<R>Collection.
// It is embedded in the spec of every collection CRD so users see the same
// filter surface regardless of resource kind. Per-resource reconcilers map
// each field to the SDK's server-side filter when available, and apply any
// remaining predicates client-side after listing.
type Predicates struct {
	// CreatedAtGte matches resources created at or after this RFC3339
	// timestamp (inclusive).
	// +optional
	CreatedAtGte *metav1.Time `json:"createdAtGte,omitempty"`

	// CreatedAtLte matches resources created at or before this RFC3339
	// timestamp (inclusive).
	// +optional
	CreatedAtLte *metav1.Time `json:"createdAtLte,omitempty"`

	// MetadataMatch retains only resources whose API metadata map contains
	// every specified key-value pair. Resources with no metadata, or missing
	// any key, are excluded. An empty map passes all resources.
	// +optional
	MetadataMatch map[string]string `json:"metadataMatch,omitempty"`

	// CELFilter is a CEL expression that must evaluate to a boolean. The
	// expression receives the JSON-decoded API response item as the variable
	// "atProvider" (map[string]any). Use bracket notation for field access:
	// atProvider["name"] == "foo" or atProvider["metadata"]["key"] == "val".
	// Resources for which the expression evaluates to false are excluded.
	// +optional
	CELFilter *string `json:"celFilter,omitempty"`
}

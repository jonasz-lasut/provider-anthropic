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
	"encoding/json"

	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
	"github.com/pkg/errors"

	betav1alpha1 "github.com/jonasz-lasut/provider-anthropic-platform/apis/beta/v1alpha1"
)

const (
	errMarshalItem   = "cannot marshal item for predicate evaluation"
	errUnmarshalItem = "cannot unmarshal item for predicate evaluation"
)

// ClientSideFilter reports whether item satisfies all client-side predicates in
// p (MetadataMatch and CELFilter). It is generic so that any JSON-serialisable
// Anthropic API response type can be filtered without boilerplate in each
// reconciler. Server-side predicates (CreatedAtGte, CreatedAtLte) are handled
// by buildListParams in each reconciler; this function covers the remainder.
func ClientSideFilter[T any](p *betav1alpha1.Predicates, item T) (bool, error) {
	if p == nil || (len(p.MetadataMatch) == 0 && p.CELFilter == nil) {
		return true, nil
	}
	data, err := json.Marshal(item)
	if err != nil {
		return false, errors.Wrap(err, errMarshalItem)
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return false, errors.Wrap(err, errUnmarshalItem)
	}
	paved := fieldpath.Pave(obj)

	if ok, err := MatchesMetadata(paved, p.MetadataMatch); err != nil || !ok {
		return ok, err
	}
	if p.CELFilter != nil {
		return PredicateDeriveFromCelQuery(*p.CELFilter, obj)
	}
	return true, nil
}

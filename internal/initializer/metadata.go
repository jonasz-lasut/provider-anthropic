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

// Package initializer holds managed.Initializer implementations used by the
// provider's reconcilers.
package initializer

import (
	"context"
	"encoding/json"
	"fmt"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	xperrors "github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
	xpresource "github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Initializer writes Crossplane's canonical identifier tags into a
// configurable map field under spec.forProvider on a managed resource. It is
// modeled on upjet's config.Tagger.
type Initializer struct {
	kube      client.Client
	fieldName string
}

// New returns an Initializer that writes the canonical Crossplane tags into
// spec.forProvider.<fieldName>. fieldName is expected to be "metadata" for
// this provider's resources, but is kept configurable for parity with upjet.
func New(kube client.Client, fieldName string) *Initializer {
	return &Initializer{kube: kube, fieldName: fieldName}
}

// Initialize merges the canonical Crossplane tags into the metadata map on
// spec.forProvider. Defaults overwrite any user-supplied value for the same
// key. The mutated object is persisted with kube.Update so the change is
// visible via kubectl and survives controller restarts.
func (i *Initializer) Initialize(ctx context.Context, mg xpresource.Managed) error {
	if sets.New(mg.GetManagementPolicies()...).
		Equal(sets.New(xpv2.ManagementActionObserve)) {
		return nil
	}

	paved, err := fieldpath.PaveObject(mg)
	if err != nil {
		return xperrors.Wrap(err, "cannot pave managed resource")
	}

	path := fmt.Sprintf("spec.forProvider.%s", i.fieldName)

	existing := map[string]string{}
	if raw, err := paved.GetValue(path); err == nil && raw != nil {
		if m, ok := raw.(map[string]any); ok {
			for k, v := range m {
				if s, ok := v.(string); ok {
					existing[k] = s
				}
			}
		}
	}
	for k, v := range xpresource.GetExternalTags(mg) {
		existing[k] = v
	}

	if err := paved.SetValue(path, existing); err != nil {
		return xperrors.Wrap(err, "cannot set spec.forProvider metadata")
	}

	pavedBytes, err := paved.MarshalJSON()
	if err != nil {
		return xperrors.Wrap(err, "cannot marshal paved object")
	}
	if err := json.Unmarshal(pavedBytes, mg); err != nil {
		return xperrors.Wrap(err, "cannot unmarshal paved object back into managed resource")
	}

	if err := i.kube.Update(ctx, mg); err != nil {
		return xperrors.Wrap(err, "cannot update managed resource with default metadata")
	}
	return nil
}

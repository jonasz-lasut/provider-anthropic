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

// Package extractors provides generic reference.ExtractValueFn implementations
// usable by `+crossplane:generate:reference:extractor=...` markers on managed
// resource types in this provider.
//
// Upjet's resource.ExtractParamPath only works on upjet-generated resources,
// because it assumes the Terraform-state layout under status. The functions
// here use fieldpath.PaveObject, which JSON-marshals any Go struct and reads a
// path string against the resulting map — so they work on handwritten managed
// resources too.
//
// Pattern adapted from
// https://github.com/grafana/crossplane-provider-grafana/blob/main/config/grafana/extractors.go
package extractors

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reference"
	xpresource "github.com/crossplane/crossplane-runtime/v2/pkg/resource"
)

// ComputedFieldExtractor returns an extractor that reads `status.atProvider.<field>`
// from the referenced managed resource. Use this for IDs and other values that
// are set by the reconciler after the external resource is created.
func ComputedFieldExtractor(field string) reference.ExtractValueFn {
	return func(mg xpresource.Managed) string {
		paved, err := fieldpath.PaveObject(mg)
		if err != nil {
			return ""
		}
		v, err := paved.GetString("status.atProvider." + field)
		if err != nil {
			return ""
		}
		return v
	}
}

// FieldExtractor returns an extractor that reads `spec.forProvider.<field>`
// from the referenced managed resource.
func FieldExtractor(field string) reference.ExtractValueFn {
	return func(mg xpresource.Managed) string {
		paved, err := fieldpath.PaveObject(mg)
		if err != nil {
			return ""
		}
		v, err := paved.GetString("spec.forProvider." + field)
		if err != nil {
			return ""
		}
		return v
	}
}

// OptionalFieldExtractor returns an extractor that prefers `spec.forProvider.<field>`
// and falls back to `status.atProvider.<field>` when the spec value is empty.
// Useful when a field may be user-supplied OR observed.
func OptionalFieldExtractor(field string) reference.ExtractValueFn {
	return func(mg xpresource.Managed) string {
		if v := FieldExtractor(field)(mg); v != "" {
			return v
		}
		return ComputedFieldExtractor(field)(mg)
	}
}

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

// Package v1beta1 contains managed resource definitions for the Anthropic
// Managed Agents beta API.
// +kubebuilder:object:generate=true
// +groupName=managedagents.anthropic.crossplane.io
package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// DeletionPolicy constants are used by resources that support both Archive and
// Delete on the Anthropic API. They are referenced by reconcilers at runtime
// and by the +kubebuilder:validation:Enum markers on the spec fields.
const (
	DeletionPolicyArchive = "Archive"
	DeletionPolicyDelete  = "Delete"
)

// Package-level variables for the API Group/Version/Kind.
var (
	// GroupVersion is the API group and version for this package.
	GroupVersion = schema.GroupVersion{
		Group:   "managedagents.anthropic.crossplane.io",
		Version: "v1beta1",
	}

	// SchemeBuilder is used to register types with a Kubernetes runtime scheme.
	// The meta v1 types (ListOptions, WatchEvent, ...) are registered once for
	// the group version here; without them the cache reflector cannot convert
	// v1.ListOptions and every List/Watch fails. Individual resources add their
	// own known types via SchemeBuilder.Register in their _types.go init().
	SchemeBuilder = runtime.NewSchemeBuilder(func(s *runtime.Scheme) error {
		metav1.AddToGroupVersion(s, GroupVersion)
		return nil
	})

	// AddToScheme adds all registered types to the supplied scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

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

// Package apis contains all API group definitions for the provider.
package apis

import (
	"k8s.io/apimachinery/pkg/runtime"

	v1beta1 "github.com/jonasz-lasut/provider-anthropic/apis/managedagents/v1beta1"
	pcv1beta1 "github.com/jonasz-lasut/provider-anthropic/apis/config/v1beta1"
)

// AddToSchemes collects the AddToScheme functions for every versioned API group.
var AddToSchemes = runtime.SchemeBuilder{
	pcv1beta1.SchemeBuilder.AddToScheme,
	v1beta1.AddToScheme,
}

// AddToScheme adds all registered types to s.
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}

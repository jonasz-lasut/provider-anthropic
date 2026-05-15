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

// Package controller registers all managed-resource controllers.
package controller

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"

	"github.com/jonasz-lasut/provider-anthropic-platform/internal/controller/agent"
	"github.com/jonasz-lasut/provider-anthropic-platform/internal/controller/environment"
	"github.com/jonasz-lasut/provider-anthropic-platform/internal/controller/providerconfig"
	"github.com/jonasz-lasut/provider-anthropic-platform/internal/controller/session"
)

// SetupProviders registers all controllers with the supplied manager.
// Each controller is gated: it will only start once its CRD is established.
func SetupProviders(mgr ctrl.Manager, o controller.Options) error {
	if err := providerconfig.SetupGated(mgr, o); err != nil {
		return err
	}
	if err := agent.SetupGated(mgr, o); err != nil {
		return err
	}
	if err := environment.SetupGated(mgr, o); err != nil {
		return err
	}
	if err := session.SetupGated(mgr, o); err != nil {
		return err
	}
	return nil
}

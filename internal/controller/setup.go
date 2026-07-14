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

// Package controller registers all managed-resource controllers.
package controller

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"

	"github.com/jonasz-lasut/provider-anthropic/internal/controller/agent"
	"github.com/jonasz-lasut/provider-anthropic/internal/controller/deployment"
	"github.com/jonasz-lasut/provider-anthropic/internal/controller/dream"
	"github.com/jonasz-lasut/provider-anthropic/internal/controller/environment"
	"github.com/jonasz-lasut/provider-anthropic/internal/controller/memorystore"
	"github.com/jonasz-lasut/provider-anthropic/internal/controller/memorystorememory"
	"github.com/jonasz-lasut/provider-anthropic/internal/controller/skill"
	"github.com/jonasz-lasut/provider-anthropic/internal/controller/providerconfig"
	"github.com/jonasz-lasut/provider-anthropic/internal/controller/session"
	"github.com/jonasz-lasut/provider-anthropic/internal/controller/vault"
	"github.com/jonasz-lasut/provider-anthropic/internal/controller/vaultcredential"
)

// SetupProviders registers all controllers with the supplied manager. Each
// controller is gated: it will only start once its CRD is established.
//
// When skipDefaultMetadata is true, the metadata-bearing controllers (Agent,
// Environment, Deployment, MemoryStore, Session, Vault, VaultCredential) are
// configured without the default-metadata initializer.
func SetupProviders(mgr ctrl.Manager, o controller.Options, skipDefaultMetadata bool) error {
	if err := providerconfig.SetupGated(mgr, o); err != nil {
		return err
	}
	if err := agent.SetupGated(mgr, o, skipDefaultMetadata); err != nil {
		return err
	}
	if err := environment.SetupGated(mgr, o, skipDefaultMetadata); err != nil {
		return err
	}
	if err := deployment.SetupGated(mgr, o, skipDefaultMetadata); err != nil {
		return err
	}
	if err := memorystore.SetupGated(mgr, o, skipDefaultMetadata); err != nil {
		return err
	}
	if err := memorystorememory.SetupGated(mgr, o); err != nil {
		return err
	}
	if err := session.SetupGated(mgr, o, skipDefaultMetadata); err != nil {
		return err
	}
	if err := vault.SetupGated(mgr, o, skipDefaultMetadata); err != nil {
		return err
	}
	if err := vaultcredential.SetupGated(mgr, o, skipDefaultMetadata); err != nil {
		return err
	}
	if err := skill.SetupGated(mgr, o); err != nil {
		return err
	}
	if err := dream.SetupGated(mgr, o); err != nil {
		return err
	}
	return nil
}

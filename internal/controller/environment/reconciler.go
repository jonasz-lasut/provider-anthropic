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

// Package environment implements the Crossplane managed reconciler for the
// Anthropic Environments beta API.
package environment

import (
	"context"
	"errors"
	"slices"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	xperrors "github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	betav1alpha1 "github.com/jonasz-lasut/provider-anthropic-platform/apis/beta/v1alpha1"
	"github.com/jonasz-lasut/provider-anthropic-platform/internal/clients"
	"github.com/jonasz-lasut/provider-anthropic-platform/internal/initializer"
)

const (
	errNotEnvironment = "managed resource is not an Environment"
	errNewClient      = "cannot build Anthropic client"
	errObserve        = "cannot observe Environment"
	errCreate         = "cannot create Environment"
	errUpdate         = "cannot update Environment"
	errDelete         = "cannot delete/archive Environment"
)

// Setup adds a controller for Environment to the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options, skipDefaultMetadata bool) error {
	name := managed.ControllerName(betav1alpha1.EnvironmentKind)

	opts := []managed.ReconcilerOption{
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithManagementPolicies(),
	}
	if !skipDefaultMetadata {
		opts = append(opts, managed.WithInitializers(initializer.New(mgr.GetClient(), "metadata")))
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&betav1alpha1.Environment{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(betav1alpha1.EnvironmentGroupVersionKind),
			opts...,
		))
}

// SetupGated registers the Environment controller to start only once the
// Environment CRD is established.
func SetupGated(mgr ctrl.Manager, o controller.Options, skipDefaultMetadata bool) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o, skipDefaultMetadata); err != nil {
			panic(err)
		}
	}, betav1alpha1.EnvironmentGroupVersionKind)
	return nil
}

// connector builds an ExternalClient for each reconcile.
type connector struct {
	kube client.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	env, ok := mg.(*betav1alpha1.Environment)
	if !ok {
		return nil, xperrors.New(errNotEnvironment)
	}

	cl, err := clients.NewClient(ctx, c.kube, env)
	if err != nil {
		return nil, xperrors.Wrap(err, errNewClient)
	}

	return &external{client: cl}, nil
}

// external implements managed.ExternalClient for Anthropic Environments.
type external struct {
	client *anthropic.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	env, ok := mg.(*betav1alpha1.Environment)
	if !ok {
		return managed.ExternalObservation{}, xperrors.New(errNotEnvironment)
	}

	// The external-name annotation holds the Anthropic environment ID once created.
	// If it still equals the k8s object name the resource has never been created.
	envID := meta.GetExternalName(env)
	if envID == env.GetName() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	resp, err := e.client.Beta.Environments.Get(ctx, envID, anthropic.BetaEnvironmentGetParams{})
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, xperrors.Wrap(err, errObserve)
	}

	// Archived environments are treated as deleted — Crossplane will re-create them.
	if resp.ArchivedAt != "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	// Populate observed state.
	env.Status.AtProvider.ID = &resp.ID
	createdAt := resp.CreatedAt
	updatedAt := resp.UpdatedAt
	env.Status.AtProvider.CreatedAt = &createdAt
	env.Status.AtProvider.UpdatedAt = &updatedAt
	if resp.ArchivedAt != "" {
		archivedAt := resp.ArchivedAt
		env.Status.AtProvider.ArchivedAt = &archivedAt
	}

	env.SetConditions(xpv1.Available())

	upToDate := isUpToDate(env, resp)
	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate,
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	env, ok := mg.(*betav1alpha1.Environment)
	if !ok {
		return managed.ExternalCreation{}, xperrors.New(errNotEnvironment)
	}

	params := buildNewParams(env.Spec.ForProvider)
	resp, err := e.client.Beta.Environments.New(ctx, params)
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}

	// Store ID in external-name (primary key) and mirror it in AtProvider
	// so cross-resource references can extract it via ComputedFieldExtractor("id").
	meta.SetExternalName(env, resp.ID)
	env.Status.AtProvider.ID = &resp.ID

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	env, ok := mg.(*betav1alpha1.Environment)
	if !ok {
		return managed.ExternalUpdate{}, xperrors.New(errNotEnvironment)
	}

	envID := meta.GetExternalName(env)
	if envID == env.GetName() {
		return managed.ExternalUpdate{}, xperrors.New("external name not yet set; skipping update")
	}

	params := buildUpdateParams(env.Spec.ForProvider)
	if _, err := e.client.Beta.Environments.Update(ctx, envID, params); err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	env, ok := mg.(*betav1alpha1.Environment)
	if !ok {
		return managed.ExternalDelete{}, xperrors.New(errNotEnvironment)
	}

	envID := meta.GetExternalName(env)
	if envID == env.GetName() {
		return managed.ExternalDelete{}, nil
	}

	policy := betav1alpha1.DeletionPolicyArchive
	if env.Spec.ForProvider.AnthropicDeletionPolicy != nil {
		policy = *env.Spec.ForProvider.AnthropicDeletionPolicy
	}

	var err error
	if policy == betav1alpha1.DeletionPolicyDelete {
		_, err = e.client.Beta.Environments.Delete(ctx, envID, anthropic.BetaEnvironmentDeleteParams{})
	} else {
		_, err = e.client.Beta.Environments.Archive(ctx, envID, anthropic.BetaEnvironmentArchiveParams{})
	}
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, xperrors.Wrap(err, errDelete)
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(_ context.Context) error { return nil }

// buildNewParams converts ForProvider into the SDK create params.
func buildNewParams(p betav1alpha1.EnvironmentParameters) anthropic.BetaEnvironmentNewParams {
	params := anthropic.BetaEnvironmentNewParams{}
	if p.Name != nil {
		params.Name = *p.Name
	}
	if p.Description != nil {
		params.Description = anthropic.String(*p.Description)
	}
	if p.Metadata != nil {
		params.Metadata = p.Metadata
	}
	if p.Config != nil {
		params.Config = buildCloudConfigParams(p.Config)
	}
	return params
}

// buildUpdateParams converts ForProvider into the SDK update params.
func buildUpdateParams(p betav1alpha1.EnvironmentParameters) anthropic.BetaEnvironmentUpdateParams {
	params := anthropic.BetaEnvironmentUpdateParams{}
	if p.Name != nil {
		params.Name = anthropic.String(*p.Name)
	}
	if p.Description != nil {
		params.Description = anthropic.String(*p.Description)
	}
	if p.Metadata != nil {
		params.Metadata = p.Metadata
	}
	if p.Config != nil {
		params.Config = buildCloudConfigParams(p.Config)
	}
	return params
}

// buildCloudConfigParams converts the CRD cloud config into the SDK params struct.
func buildCloudConfigParams(cfg *betav1alpha1.EnvironmentCloudConfig) anthropic.BetaCloudConfigParams {
	params := anthropic.BetaCloudConfigParams{}
	if cfg.Networking != nil {
		netType := ""
		if cfg.Networking.Type != nil {
			netType = *cfg.Networking.Type
		}
		switch netType {
		case "unrestricted":
			unr := anthropic.NewBetaUnrestrictedNetworkParam()
			params.Networking = anthropic.BetaCloudConfigParamsNetworkingUnion{
				OfUnrestricted: &unr,
			}
		case "limited":
			limited := anthropic.BetaLimitedNetworkParams{
				Type:         "limited",
				AllowedHosts: cfg.Networking.AllowedHosts,
			}
			if cfg.Networking.AllowMCPServers != nil {
				limited.AllowMCPServers = anthropic.Bool(*cfg.Networking.AllowMCPServers)
			}
			if cfg.Networking.AllowPackageManagers != nil {
				limited.AllowPackageManagers = anthropic.Bool(*cfg.Networking.AllowPackageManagers)
			}
			params.Networking = anthropic.BetaCloudConfigParamsNetworkingUnion{
				OfLimited: &limited,
			}
		}
	}
	if cfg.Packages != nil {
		params.Packages = anthropic.BetaPackagesParams{
			Apt:   cfg.Packages.Apt,
			Cargo: cfg.Packages.Cargo,
			Gem:   cfg.Packages.Gem,
			Go:    cfg.Packages.Go,
			Npm:   cfg.Packages.Npm,
			Pip:   cfg.Packages.Pip,
		}
	}
	return params
}

// isUpToDate compares the desired state with the observed environment.
func isUpToDate(env *betav1alpha1.Environment, resp *anthropic.BetaEnvironment) bool {
	p := env.Spec.ForProvider

	if p.Name != nil && *p.Name != resp.Name {
		return false
	}
	if p.Description != nil && *p.Description != resp.Description {
		return false
	}

	// Metadata
	if len(p.Metadata) != len(resp.Metadata) {
		return false
	}
	for k, v := range p.Metadata {
		if resp.Metadata[k] != v {
			return false
		}
	}

	// Config (only checked when the user specifies it)
	if p.Config != nil {
		if p.Config.Networking != nil {
			n := p.Config.Networking
			if n.Type != nil && *n.Type != resp.Config.Networking.Type {
				return false
			}
			if n.Type != nil && *n.Type == "limited" {
				if n.AllowMCPServers != nil && *n.AllowMCPServers != resp.Config.Networking.AllowMCPServers {
					return false
				}
				if n.AllowPackageManagers != nil && *n.AllowPackageManagers != resp.Config.Networking.AllowPackageManagers {
					return false
				}
				if !slices.Equal(n.AllowedHosts, resp.Config.Networking.AllowedHosts) {
					return false
				}
			}
		}
		if p.Config.Packages != nil {
			pkgs := p.Config.Packages
			if !slices.Equal(pkgs.Apt, resp.Config.Packages.Apt) {
				return false
			}
			if !slices.Equal(pkgs.Cargo, resp.Config.Packages.Cargo) {
				return false
			}
			if !slices.Equal(pkgs.Gem, resp.Config.Packages.Gem) {
				return false
			}
			if !slices.Equal(pkgs.Go, resp.Config.Packages.Go) {
				return false
			}
			if !slices.Equal(pkgs.Npm, resp.Config.Packages.Npm) {
				return false
			}
			if !slices.Equal(pkgs.Pip, resp.Config.Packages.Pip) {
				return false
			}
		}
	}

	return true
}

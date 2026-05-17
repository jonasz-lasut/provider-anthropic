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

// Package vault implements the Crossplane managed reconciler for the
// Anthropic Vault beta API.
package vault

import (
	"context"
	"errors"
	"time"

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
)

const (
	errNotVault  = "managed resource is not a Vault"
	errNewClient = "cannot build Anthropic client"
	errObserve   = "cannot observe Vault"
	errCreate    = "cannot create Vault"
	errUpdate    = "cannot update Vault"
	errDelete    = "cannot delete/archive Vault"
)

// Setup adds a controller for Vault to the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(betav1alpha1.VaultKind)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&betav1alpha1.Vault{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(betav1alpha1.VaultGroupVersionKind),
			managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
			managed.WithLogger(o.Logger.WithValues("controller", name)),
			managed.WithPollInterval(o.PollInterval),
			managed.WithManagementPolicies(),
		))
}

// SetupGated registers the Vault controller to start only once the Vault CRD
// is established.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(err)
		}
	}, betav1alpha1.VaultGroupVersionKind)
	return nil
}

// connector builds an ExternalClient for each reconcile.
type connector struct {
	kube client.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	v, ok := mg.(*betav1alpha1.Vault)
	if !ok {
		return nil, xperrors.New(errNotVault)
	}

	cl, err := clients.NewClient(ctx, c.kube, v)
	if err != nil {
		return nil, xperrors.Wrap(err, errNewClient)
	}

	return &external{client: cl}, nil
}

// external implements managed.ExternalClient for Anthropic Vaults.
type external struct {
	client *anthropic.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	v, ok := mg.(*betav1alpha1.Vault)
	if !ok {
		return managed.ExternalObservation{}, xperrors.New(errNotVault)
	}

	vID := meta.GetExternalName(v)
	if vID == v.GetName() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	resp, err := e.client.Beta.Vaults.Get(ctx, vID, anthropic.BetaVaultGetParams{})
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, xperrors.Wrap(err, errObserve)
	}

	// Archived vaults are treated as deleted — Crossplane will re-create them.
	if !resp.ArchivedAt.IsZero() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	v.Status.AtProvider.ID = &resp.ID
	createdAt := resp.CreatedAt.Format(time.RFC3339)
	updatedAt := resp.UpdatedAt.Format(time.RFC3339)
	v.Status.AtProvider.CreatedAt = &createdAt
	v.Status.AtProvider.UpdatedAt = &updatedAt
	if !resp.ArchivedAt.IsZero() {
		archivedAt := resp.ArchivedAt.Format(time.RFC3339)
		v.Status.AtProvider.ArchivedAt = &archivedAt
	}

	v.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isUpToDate(v, resp),
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	v, ok := mg.(*betav1alpha1.Vault)
	if !ok {
		return managed.ExternalCreation{}, xperrors.New(errNotVault)
	}

	params := buildNewParams(v.Spec.ForProvider)
	resp, err := e.client.Beta.Vaults.New(ctx, params)
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}

	meta.SetExternalName(v, resp.ID)
	v.Status.AtProvider.ID = &resp.ID

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	v, ok := mg.(*betav1alpha1.Vault)
	if !ok {
		return managed.ExternalUpdate{}, xperrors.New(errNotVault)
	}

	vID := meta.GetExternalName(v)
	if vID == v.GetName() {
		return managed.ExternalUpdate{}, xperrors.New("external name not yet set; skipping update")
	}

	params := buildUpdateParams(v.Spec.ForProvider)
	if _, err := e.client.Beta.Vaults.Update(ctx, vID, params); err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	v, ok := mg.(*betav1alpha1.Vault)
	if !ok {
		return managed.ExternalDelete{}, xperrors.New(errNotVault)
	}

	vID := meta.GetExternalName(v)
	if vID == v.GetName() {
		return managed.ExternalDelete{}, nil
	}

	policy := betav1alpha1.DeletionPolicyArchive
	if v.Spec.ForProvider.AnthropicDeletionPolicy != nil {
		policy = *v.Spec.ForProvider.AnthropicDeletionPolicy
	}

	var err error
	if policy == betav1alpha1.DeletionPolicyDelete {
		_, err = e.client.Beta.Vaults.Delete(ctx, vID, anthropic.BetaVaultDeleteParams{})
	} else {
		_, err = e.client.Beta.Vaults.Archive(ctx, vID, anthropic.BetaVaultArchiveParams{})
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
func buildNewParams(p betav1alpha1.VaultParameters) anthropic.BetaVaultNewParams {
	params := anthropic.BetaVaultNewParams{}
	if p.DisplayName != nil {
		params.DisplayName = *p.DisplayName
	}
	if p.Metadata != nil {
		params.Metadata = p.Metadata
	}
	return params
}

// buildUpdateParams converts ForProvider into the SDK update params.
func buildUpdateParams(p betav1alpha1.VaultParameters) anthropic.BetaVaultUpdateParams {
	params := anthropic.BetaVaultUpdateParams{}
	if p.DisplayName != nil {
		params.DisplayName = anthropic.String(*p.DisplayName)
	}
	if p.Metadata != nil {
		params.Metadata = p.Metadata
	}
	return params
}

// isUpToDate compares the desired state with the observed vault.
func isUpToDate(v *betav1alpha1.Vault, resp *anthropic.BetaManagedAgentsVault) bool {
	p := v.Spec.ForProvider

	if p.DisplayName != nil && *p.DisplayName != resp.DisplayName {
		return false
	}

	if len(p.Metadata) != len(resp.Metadata) {
		return false
	}
	for k, val := range p.Metadata {
		if resp.Metadata[k] != val {
			return false
		}
	}

	return true
}

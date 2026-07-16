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

// Package vault implements the Crossplane managed reconciler for the
// Anthropic Vault beta API.
package vault

import (
	"context"
	"encoding/json"
	"errors"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	xperrors "github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	v1beta1 "github.com/jonasz-lasut/provider-anthropic/apis/managedagents/v1beta1"
	"github.com/jonasz-lasut/provider-anthropic/internal/clients"
	"github.com/jonasz-lasut/provider-anthropic/internal/initializer"
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
func Setup(mgr ctrl.Manager, o controller.Options, skipDefaultMetadata bool) error {
	name := managed.ControllerName(v1beta1.VaultKind)

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
		For(&v1beta1.Vault{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1beta1.VaultGroupVersionKind),
			opts...,
		))
}

// SetupGated registers the Vault controller to start only once the Vault CRD
// is established.
func SetupGated(mgr ctrl.Manager, o controller.Options, skipDefaultMetadata bool) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o, skipDefaultMetadata); err != nil {
			panic(err)
		}
	}, v1beta1.VaultGroupVersionKind)
	return nil
}

// connector builds an ExternalClient for each reconcile.
type connector struct {
	kube client.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	v, ok := mg.(*v1beta1.Vault)
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
	v, ok := mg.(*v1beta1.Vault)
	if !ok {
		return managed.ExternalObservation{}, xperrors.New(errNotVault)
	}

	// Crossplane seeds external-name with the k8s object name before Create runs.
	// Some Anthropic APIs return 400 (not 404) for non-prefixed IDs, so detect
	// "not yet created" by comparing against the k8s name rather than checking empty.
	vID := meta.GetExternalName(v)
	if vID == "" || vID == v.GetName() {
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

	v.FromAnthropicObservation(*resp)

	v.SetConditions(xpv2.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isUpToDate(v),
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	v, ok := mg.(*v1beta1.Vault)
	if !ok {
		return managed.ExternalCreation{}, xperrors.New(errNotVault)
	}

	params := v.ToAnthropicNew()
	resp, err := e.client.Beta.Vaults.New(ctx, params)
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}

	meta.SetExternalName(v, resp.ID)
	v.Status.AtProvider.ID = &resp.ID

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	v, ok := mg.(*v1beta1.Vault)
	if !ok {
		return managed.ExternalUpdate{}, xperrors.New(errNotVault)
	}

	vID := meta.GetExternalName(v)
	if vID == "" || vID == v.GetName() {
		return managed.ExternalUpdate{}, xperrors.New("external name not yet set; skipping update")
	}

	params := v.ToAnthropicUpdate()
	if _, err := e.client.Beta.Vaults.Update(ctx, vID, params); err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	v, ok := mg.(*v1beta1.Vault)
	if !ok {
		return managed.ExternalDelete{}, xperrors.New(errNotVault)
	}

	vID := meta.GetExternalName(v)
	if vID == "" || vID == v.GetName() {
		return managed.ExternalDelete{}, nil
	}

	policy := v1beta1.DeletionPolicyArchive
	if v.Spec.ForProvider.AnthropicDeletionPolicy != nil {
		policy = *v.Spec.ForProvider.AnthropicDeletionPolicy
	}

	var err error
	if policy == v1beta1.DeletionPolicyDelete {
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

// isUpToDate compares the desired state with the observed vault.
// isUpToDate performs a structured diff between spec.forProvider and
// status.atProvider, skipping nil ForProvider fields and ForProvider-only
// fields that have no AtProvider counterpart (e.g. AnthropicDeletionPolicy).
func isUpToDate(v *v1beta1.Vault) bool {
	fpRaw, err := json.Marshal(v.Spec.ForProvider)
	if err != nil {
		return true
	}
	apRaw, err := json.Marshal(v.Status.AtProvider)
	if err != nil {
		return true
	}
	var fp, ap map[string]any
	if err := json.Unmarshal(fpRaw, &fp); err != nil {
		return true
	}
	if err := json.Unmarshal(apRaw, &ap); err != nil {
		return true
	}
	return clients.IsSubsetEqual(fp, ap)
}

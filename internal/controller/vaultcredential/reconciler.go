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

// Package vaultcredential implements the Crossplane managed reconciler for
// the Anthropic VaultCredential beta API.
package vaultcredential

import (
	"context"
	"encoding/json"
	"errors"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
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
	errNotVaultCredential = "managed resource is not a VaultCredential"
	errNewClient          = "cannot build Anthropic client"
	errObserve            = "cannot observe VaultCredential"
	errCreate             = "cannot create VaultCredential"
	errUpdate             = "cannot update VaultCredential"
	errDelete             = "cannot delete/archive VaultCredential"
	errMissingVault       = "spec.forProvider.vaultId not resolved"
)

// Setup adds a controller for VaultCredential to the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options, skipDefaultMetadata bool) error {
	name := managed.ControllerName(v1beta1.VaultCredentialKind)

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
		For(&v1beta1.VaultCredential{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1beta1.VaultCredentialGroupVersionKind),
			opts...,
		))
}

// SetupGated registers the VaultCredential controller to start only once the
// VaultCredential CRD is established.
func SetupGated(mgr ctrl.Manager, o controller.Options, skipDefaultMetadata bool) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o, skipDefaultMetadata); err != nil {
			panic(err)
		}
	}, v1beta1.VaultCredentialGroupVersionKind)
	return nil
}

// connector builds an ExternalClient for each reconcile.
type connector struct {
	kube client.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	vc, ok := mg.(*v1beta1.VaultCredential)
	if !ok {
		return nil, xperrors.New(errNotVaultCredential)
	}

	cl, err := clients.NewClient(ctx, c.kube, vc)
	if err != nil {
		return nil, xperrors.Wrap(err, errNewClient)
	}

	return &external{client: cl, kube: c.kube}, nil
}

// external implements managed.ExternalClient for Anthropic VaultCredentials.
type external struct {
	client *anthropic.Client
	kube   client.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	vc, ok := mg.(*v1beta1.VaultCredential)
	if !ok {
		return managed.ExternalObservation{}, xperrors.New(errNotVaultCredential)
	}

	// Crossplane seeds external-name with the k8s object name before Create runs.
	// Some Anthropic APIs return 400 (not 404) for non-prefixed IDs, so detect
	// "not yet created" by comparing against the k8s name rather than checking empty.
	credID := meta.GetExternalName(vc)
	if credID == "" || credID == vc.GetName() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	if vc.Spec.ForProvider.VaultID == nil || *vc.Spec.ForProvider.VaultID == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	resp, err := e.client.Beta.Vaults.Credentials.Get(ctx, credID, anthropic.BetaVaultCredentialGetParams{
		VaultID: *vc.Spec.ForProvider.VaultID,
	})
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, xperrors.Wrap(err, errObserve)
	}

	// Archived credentials are treated as deleted — Crossplane will re-create them.
	if !resp.ArchivedAt.IsZero() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	vc.FromAnthropicObservation(*resp)

	vc.SetConditions(xpv1.Available())

	convCtx, err := resolveVCContext(ctx, e.kube, vc.Spec.ForProvider.Auth, vc.GetNamespace())
	if err != nil {
		return managed.ExternalObservation{}, xperrors.Wrap(err, errObserve)
	}
	return managed.ExternalObservation{
		ResourceExists:    true,
		ResourceUpToDate:  isUpToDate(vc),
		ConnectionDetails: convCtx.ToConnectionDetails(),
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	vc, ok := mg.(*v1beta1.VaultCredential)
	if !ok {
		return managed.ExternalCreation{}, xperrors.New(errNotVaultCredential)
	}

	if vc.Spec.ForProvider.VaultID == nil || *vc.Spec.ForProvider.VaultID == "" {
		return managed.ExternalCreation{}, xperrors.New(errMissingVault)
	}

	convCtx, err := resolveVCContext(ctx, e.kube, vc.Spec.ForProvider.Auth, vc.GetNamespace())
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}
	params, err := vc.ToAnthropicNew(convCtx)
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}
	resp, err := e.client.Beta.Vaults.Credentials.New(ctx, *vc.Spec.ForProvider.VaultID, params)
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}

	meta.SetExternalName(vc, resp.ID)
	vc.Status.AtProvider.ID = &resp.ID
	vc.Status.AtProvider.VaultID = &resp.VaultID

	return managed.ExternalCreation{ConnectionDetails: convCtx.ToConnectionDetails()}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	vc, ok := mg.(*v1beta1.VaultCredential)
	if !ok {
		return managed.ExternalUpdate{}, xperrors.New(errNotVaultCredential)
	}

	credID := meta.GetExternalName(vc)
	if credID == "" || credID == vc.GetName() {
		return managed.ExternalUpdate{}, xperrors.New("external name not yet set; skipping update")
	}

	if vc.Spec.ForProvider.VaultID == nil || *vc.Spec.ForProvider.VaultID == "" {
		return managed.ExternalUpdate{}, xperrors.New(errMissingVault)
	}

	convCtx, err := resolveVCContext(ctx, e.kube, vc.Spec.ForProvider.Auth, vc.GetNamespace())
	if err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}
	params, err := vc.ToAnthropicUpdate(convCtx)
	if err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}
	if _, err := e.client.Beta.Vaults.Credentials.Update(ctx, credID, params); err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}

	return managed.ExternalUpdate{ConnectionDetails: convCtx.ToConnectionDetails()}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	vc, ok := mg.(*v1beta1.VaultCredential)
	if !ok {
		return managed.ExternalDelete{}, xperrors.New(errNotVaultCredential)
	}

	credID := meta.GetExternalName(vc)
	if credID == "" || credID == vc.GetName() {
		return managed.ExternalDelete{}, nil
	}

	if vc.Spec.ForProvider.VaultID == nil || *vc.Spec.ForProvider.VaultID == "" {
		// Without a resolved vault ID we cannot target the API; treat as no-op.
		return managed.ExternalDelete{}, nil
	}

	policy := v1beta1.DeletionPolicyArchive
	if vc.Spec.ForProvider.AnthropicDeletionPolicy != nil {
		policy = *vc.Spec.ForProvider.AnthropicDeletionPolicy
	}

	var err error
	if policy == v1beta1.DeletionPolicyDelete {
		_, err = e.client.Beta.Vaults.Credentials.Delete(ctx, credID, anthropic.BetaVaultCredentialDeleteParams{
			VaultID: *vc.Spec.ForProvider.VaultID,
		})
	} else {
		_, err = e.client.Beta.Vaults.Credentials.Archive(ctx, credID, anthropic.BetaVaultCredentialArchiveParams{
			VaultID: *vc.Spec.ForProvider.VaultID,
		})
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

// resolveVCContext pre-resolves all Kubernetes secrets referenced by a
// VaultCredential auth spec into a flat context struct ready for conversion.
func resolveVCContext(ctx context.Context, kube client.Client, a v1beta1.VaultCredentialAuth, namespace string) (*v1beta1.VaultCredentialConversionContext, error) {
	convCtx := &v1beta1.VaultCredentialConversionContext{}
	bearerToken, err := clients.ResolveLocalSecretKey(ctx, kube, a.TokenSecretRef, namespace)
	if err != nil {
		return nil, err
	}
	convCtx.BearerToken = bearerToken
	accessToken, err := clients.ResolveLocalSecretKey(ctx, kube, a.AccessTokenSecretRef, namespace)
	if err != nil {
		return nil, err
	}
	convCtx.AccessToken = accessToken
	secretValue, err := clients.ResolveLocalSecretKey(ctx, kube, a.SecretValueSecretRef, namespace)
	if err != nil {
		return nil, err
	}
	convCtx.SecretValue = secretValue
	if a.Refresh != nil {
		refreshToken, err := clients.ResolveLocalSecretKey(ctx, kube, a.Refresh.RefreshTokenSecretRef, namespace)
		if err != nil {
			return nil, err
		}
		convCtx.RefreshToken = refreshToken
		clientSecret, err := clients.ResolveLocalSecretKey(ctx, kube, a.Refresh.TokenEndpointAuth.ClientSecretSecretRef, namespace)
		if err != nil {
			return nil, err
		}
		convCtx.ClientSecret = clientSecret
	}
	return convCtx, nil
}

// isUpToDate compares the desired state with the observed credential. The
// auth payload itself (token, access token) is write-only and never returned,
// so we cannot diff it — callers who want to rotate must touch the spec to
// trigger an Update.
// isUpToDate performs a structured diff between spec.forProvider and
// status.atProvider. Token-bearing ForProvider auth sub-fields (tokenSecretRef,
// accessTokenSecretRef, refresh) are absent from AtProvider and therefore
// skipped automatically — credential values are write-only.
func isUpToDate(vc *v1beta1.VaultCredential) bool {
	fpRaw, err := json.Marshal(vc.Spec.ForProvider)
	if err != nil {
		return true
	}
	apRaw, err := json.Marshal(vc.Status.AtProvider)
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

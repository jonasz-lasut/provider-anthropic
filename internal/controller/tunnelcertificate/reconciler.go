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

// Package tunnelcertificate implements the Crossplane managed reconciler for
// the Anthropic tunnel-certificates beta API. A TunnelCertificate is a
// sub-resource of a Tunnel (its methods take the parent tunnel ID) and is
// immutable after creation, so Update is a no-op and the resource is always
// reported up to date.
package tunnelcertificate

import (
	"context"
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
)

const (
	errNotTunnelCertificate = "managed resource is not a TunnelCertificate"
	errNewClient            = "cannot build Anthropic client"
	errObserve              = "cannot observe TunnelCertificate"
	errCreate               = "cannot create TunnelCertificate"
	errDelete               = "cannot delete/archive TunnelCertificate"
	errMissingTunnel        = "spec.forProvider.tunnelId not resolved"
)

// Setup adds a controller for TunnelCertificate to the supplied manager.
// TunnelCertificate has no forProvider.metadata field, so no default-metadata
// initializer is registered.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1beta1.TunnelCertificateKind)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1beta1.TunnelCertificate{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1beta1.TunnelCertificateGroupVersionKind),
			managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
			managed.WithLogger(o.Logger.WithValues("controller", name)),
			managed.WithPollInterval(o.PollInterval),
			managed.WithManagementPolicies(),
		))
}

// SetupGated registers the TunnelCertificate controller to start only once the
// TunnelCertificate CRD is established.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(err)
		}
	}, v1beta1.TunnelCertificateGroupVersionKind)
	return nil
}

// connector builds an ExternalClient for each reconcile.
type connector struct {
	kube client.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	tc, ok := mg.(*v1beta1.TunnelCertificate)
	if !ok {
		return nil, xperrors.New(errNotTunnelCertificate)
	}

	cl, err := clients.NewClient(ctx, c.kube, tc)
	if err != nil {
		return nil, xperrors.Wrap(err, errNewClient)
	}

	return &external{client: cl, kube: c.kube}, nil
}

// external implements managed.ExternalClient for Anthropic TunnelCertificates.
type external struct {
	client *anthropic.Client
	kube   client.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	tc, ok := mg.(*v1beta1.TunnelCertificate)
	if !ok {
		return managed.ExternalObservation{}, xperrors.New(errNotTunnelCertificate)
	}

	// Crossplane seeds external-name with the k8s object name before Create runs.
	// Some Anthropic APIs return 400 (not 404) for non-prefixed IDs, so detect
	// "not yet created" by comparing against the k8s name rather than checking empty.
	certID := meta.GetExternalName(tc)
	if certID == "" || certID == tc.GetName() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	// The parent tunnel address is required to target the API. If it has not
	// resolved yet, report absence and let a later loop retry once the
	// reference is populated.
	if tc.Spec.ForProvider.TunnelID == nil || *tc.Spec.ForProvider.TunnelID == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	resp, err := e.client.Beta.Tunnels.Certificates.Get(ctx, certID, anthropic.BetaTunnelCertificateGetParams{
		TunnelID: *tc.Spec.ForProvider.TunnelID,
	})
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, xperrors.Wrap(err, errObserve)
	}

	// Archived certificates are treated as deleted — Crossplane will re-create them.
	if !resp.ArchivedAt.IsZero() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	tc.FromAnthropicObservation(*resp)

	tc.SetConditions(xpv2.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true, // immutable: no update endpoint, never drifts.
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	tc, ok := mg.(*v1beta1.TunnelCertificate)
	if !ok {
		return managed.ExternalCreation{}, xperrors.New(errNotTunnelCertificate)
	}

	if tc.Spec.ForProvider.TunnelID == nil || *tc.Spec.ForProvider.TunnelID == "" {
		return managed.ExternalCreation{}, xperrors.New(errMissingTunnel)
	}

	pem, err := clients.ResolveLocalSecretKey(ctx, e.kube, tc.Spec.ForProvider.CaCertificatePemSecretRef, tc.GetNamespace())
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}
	convCtx := &v1beta1.TunnelCertificateConversionContext{CaCertificatePem: pem}
	params := tc.ToAnthropicNew(convCtx)

	resp, err := e.client.Beta.Tunnels.Certificates.New(ctx, *tc.Spec.ForProvider.TunnelID, params)
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}

	meta.SetExternalName(tc, resp.ID)
	tc.Status.AtProvider.ID = &resp.ID
	tc.Status.AtProvider.TunnelID = &resp.TunnelID

	return managed.ExternalCreation{}, nil
}

// Update is a no-op. A TunnelCertificate is immutable after creation: the
// Anthropic API exposes no update endpoint, and Observe always reports the
// resource up to date, so this method is never expected to be called.
func (e *external) Update(_ context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	if _, ok := mg.(*v1beta1.TunnelCertificate); !ok {
		return managed.ExternalUpdate{}, xperrors.New(errNotTunnelCertificate)
	}
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	tc, ok := mg.(*v1beta1.TunnelCertificate)
	if !ok {
		return managed.ExternalDelete{}, xperrors.New(errNotTunnelCertificate)
	}

	certID := meta.GetExternalName(tc)
	if certID == "" || certID == tc.GetName() {
		return managed.ExternalDelete{}, nil
	}

	if tc.Spec.ForProvider.TunnelID == nil || *tc.Spec.ForProvider.TunnelID == "" {
		// Without a resolved parent we cannot target the API; treat as no-op.
		return managed.ExternalDelete{}, nil
	}

	// Certificates support Archive only (no hard Delete).
	_, err := e.client.Beta.Tunnels.Certificates.Archive(ctx, certID, anthropic.BetaTunnelCertificateArchiveParams{
		TunnelID: *tc.Spec.ForProvider.TunnelID,
	})
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

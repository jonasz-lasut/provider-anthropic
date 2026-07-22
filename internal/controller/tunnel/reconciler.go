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

// Package tunnel implements the Crossplane managed reconciler for the Anthropic
// Tunnels beta API. A Tunnel is an MCP tunnel that is immutable after creation
// (the API exposes no update endpoint), so Update is a no-op and the resource
// is always reported up to date. Its connector token is fetched via RevealToken
// and published as a connection detail on every reconcile.
package tunnel

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
	errNotTunnel = "managed resource is not a Tunnel"
	errNewClient = "cannot build Anthropic client"
	errObserve   = "cannot observe Tunnel"
	errReveal    = "cannot reveal Tunnel token"
	errCreate    = "cannot create Tunnel"
	errDelete    = "cannot delete/archive Tunnel"
)

// Setup adds a controller for Tunnel to the supplied manager. Tunnel has no
// forProvider.metadata field, so no default-metadata initializer is registered.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1beta1.TunnelKind)

	opts := []managed.ReconcilerOption{
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithManagementPolicies(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1beta1.Tunnel{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1beta1.TunnelGroupVersionKind),
			opts...,
		))
}

// SetupGated registers the Tunnel controller to start only once the Tunnel CRD
// is established.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(err)
		}
	}, v1beta1.TunnelGroupVersionKind)
	return nil
}

// connector builds an ExternalClient for each reconcile.
type connector struct {
	kube client.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	t, ok := mg.(*v1beta1.Tunnel)
	if !ok {
		return nil, xperrors.New(errNotTunnel)
	}

	cl, err := clients.NewClient(ctx, c.kube, t)
	if err != nil {
		return nil, xperrors.Wrap(err, errNewClient)
	}

	return &external{client: cl}, nil
}

// external implements managed.ExternalClient for Anthropic Tunnels.
type external struct {
	client *anthropic.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	t, ok := mg.(*v1beta1.Tunnel)
	if !ok {
		return managed.ExternalObservation{}, xperrors.New(errNotTunnel)
	}

	// Crossplane seeds external-name with the k8s object name before Create runs.
	// Some Anthropic APIs return 400 (not 404) for non-prefixed IDs, so detect
	// "not yet created" by comparing against the k8s name rather than checking empty.
	tunnelID := meta.GetExternalName(t)
	if tunnelID == "" || tunnelID == t.GetName() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	resp, err := e.client.Beta.Tunnels.Get(ctx, tunnelID, anthropic.BetaTunnelGetParams{})
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, xperrors.Wrap(err, errObserve)
	}

	// Archived tunnels are treated as deleted — Crossplane will re-create them.
	if !resp.ArchivedAt.IsZero() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	t.FromAnthropicObservation(*resp)

	t.SetConditions(xpv2.Available())

	// Reveal the connector token on every loop so the published connection
	// secret stays current. The value is stable until the token is rotated.
	convCtx, err := e.revealToken(ctx, tunnelID, resp.Domain)
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	return managed.ExternalObservation{
		ResourceExists:    true,
		ResourceUpToDate:  true, // immutable: no update endpoint, never drifts.
		ConnectionDetails: convCtx.ToConnectionDetails(),
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	t, ok := mg.(*v1beta1.Tunnel)
	if !ok {
		return managed.ExternalCreation{}, xperrors.New(errNotTunnel)
	}

	params := t.ToAnthropicNew()
	resp, err := e.client.Beta.Tunnels.New(ctx, params)
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}

	// Store ID in external-name (primary key) and mirror it in AtProvider so
	// cross-resource references can extract it via ComputedFieldExtractor("id").
	meta.SetExternalName(t, resp.ID)
	t.Status.AtProvider.ID = &resp.ID

	// A fresh tunnel already has a connector token; publish it immediately.
	convCtx, err := e.revealToken(ctx, resp.ID, resp.Domain)
	if err != nil {
		return managed.ExternalCreation{}, err
	}

	return managed.ExternalCreation{ConnectionDetails: convCtx.ToConnectionDetails()}, nil
}

// Update is a no-op. A Tunnel is immutable after creation: the Anthropic API
// exposes no update endpoint, and Observe always reports the resource up to
// date, so this method is never expected to be called.
func (e *external) Update(_ context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	if _, ok := mg.(*v1beta1.Tunnel); !ok {
		return managed.ExternalUpdate{}, xperrors.New(errNotTunnel)
	}
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	t, ok := mg.(*v1beta1.Tunnel)
	if !ok {
		return managed.ExternalDelete{}, xperrors.New(errNotTunnel)
	}

	tunnelID := meta.GetExternalName(t)
	if tunnelID == "" || tunnelID == t.GetName() {
		return managed.ExternalDelete{}, nil
	}

	// Tunnels support Archive only (no hard Delete).
	if _, err := e.client.Beta.Tunnels.Archive(ctx, tunnelID, anthropic.BetaTunnelArchiveParams{}); err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, xperrors.Wrap(err, errDelete)
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(_ context.Context) error { return nil }

// revealToken fetches the tunnel's connector token and wraps it, together with
// the tunnel's domain, in a conversion context ready to publish as connection
// details.
func (e *external) revealToken(ctx context.Context, tunnelID, domain string) (*v1beta1.TunnelConversionContext, error) {
	tok, err := e.client.Beta.Tunnels.RevealToken(ctx, tunnelID, anthropic.BetaTunnelRevealTokenParams{})
	if err != nil {
		return nil, xperrors.Wrap(err, errReveal)
	}
	return &v1beta1.TunnelConversionContext{
		Domain:      domain,
		TunnelToken: tok.TunnelToken,
		TokenID:     tok.ID,
	}, nil
}

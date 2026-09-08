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

// Package invite implements the Crossplane managed reconciler for the
// Anthropic Admin API Invite resource.
package invite

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

	v1beta1 "github.com/jonasz-lasut/provider-anthropic/apis/organization/v1beta1"
	"github.com/jonasz-lasut/provider-anthropic/internal/clients"
)

const (
	errNotInvite = "managed resource is not an Invite"
	errNewClient = "cannot build Anthropic client"
	errObserve   = "cannot observe Invite"
	errCreate    = "cannot create Invite"
	errDelete    = "cannot delete Invite"
)

// Setup adds a controller for Invite to the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1beta1.InviteKind)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1beta1.Invite{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1beta1.InviteGroupVersionKind),
			managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
			managed.WithLogger(o.Logger.WithValues("controller", name)),
			managed.WithPollInterval(o.PollInterval),
			managed.WithManagementPolicies(),
		))
}

// SetupGated registers the Invite controller to start only once the Invite
// CRD is established.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(err)
		}
	}, v1beta1.InviteGroupVersionKind)
	return nil
}

// connector builds an ExternalClient for each reconcile.
type connector struct {
	kube client.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	inv, ok := mg.(*v1beta1.Invite)
	if !ok {
		return nil, xperrors.New(errNotInvite)
	}

	cl, err := clients.NewClient(ctx, c.kube, inv)
	if err != nil {
		return nil, xperrors.Wrap(err, errNewClient)
	}

	return &external{client: cl}, nil
}

// external implements managed.ExternalClient for Anthropic Invites. Invites
// are create-only: there is no update endpoint and every field is immutable.
type external struct {
	client *anthropic.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	inv, ok := mg.(*v1beta1.Invite)
	if !ok {
		return managed.ExternalObservation{}, xperrors.New(errNotInvite)
	}

	// Crossplane seeds external-name with the k8s object name before Create runs.
	invID := meta.GetExternalName(inv)
	if invID == "" || invID == inv.GetName() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	resp, err := e.client.Beta.Organization.Invites.Get(ctx, invID)
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, xperrors.Wrap(err, errObserve)
	}

	// A revoked invite is gone as far as the desired state is concerned;
	// Crossplane re-issues it. Accepted and expired invites stay observed.
	if resp.Status == anthropic.BetaOrganizationInviteStatusDeleted {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	inv.FromAnthropicObservation(*resp)

	inv.SetConditions(xpv2.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true, // immutable: no update endpoint, never drifts.
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	inv, ok := mg.(*v1beta1.Invite)
	if !ok {
		return managed.ExternalCreation{}, xperrors.New(errNotInvite)
	}

	resp, err := e.client.Beta.Organization.Invites.New(ctx, inv.ToAnthropicNew())
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}

	meta.SetExternalName(inv, resp.ID)
	inv.Status.AtProvider.ID = &resp.ID

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(_ context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	if _, ok := mg.(*v1beta1.Invite); !ok {
		return managed.ExternalUpdate{}, xperrors.New(errNotInvite)
	}
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	inv, ok := mg.(*v1beta1.Invite)
	if !ok {
		return managed.ExternalDelete{}, xperrors.New(errNotInvite)
	}

	invID := meta.GetExternalName(inv)
	if invID == "" || invID == inv.GetName() {
		return managed.ExternalDelete{}, nil
	}

	_, err := e.client.Beta.Organization.Invites.Delete(ctx, invID)
	if err == nil {
		return managed.ExternalDelete{}, nil
	}
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
		return managed.ExternalDelete{}, nil
	}
	// Only a pending invite can be revoked. If the invite has already been
	// accepted, expired, or revoked there is nothing left to delete.
	if cur, getErr := e.client.Beta.Organization.Invites.Get(ctx, invID); getErr == nil && cur.Status != anthropic.BetaOrganizationInviteStatusPending {
		return managed.ExternalDelete{}, nil
	}
	return managed.ExternalDelete{}, xperrors.Wrap(err, errDelete)
}

func (e *external) Disconnect(_ context.Context) error { return nil }

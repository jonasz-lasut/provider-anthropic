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

// Package dream implements the Crossplane managed reconciler for the
// Anthropic Dreams beta API. A Dream is an asynchronous memory-consolidation
// job that is immutable after creation (the API exposes no update endpoint),
// so Update is a no-op and the resource is always reported up to date.
package dream

import (
	"context"
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
)

const (
	errNotDream  = "managed resource is not a Dream"
	errNewClient = "cannot build Anthropic client"
	errObserve   = "cannot observe Dream"
	errCreate    = "cannot create Dream"
	errDelete    = "cannot delete/archive Dream"
)

// Setup adds a controller for Dream to the supplied manager. Dream has no
// forProvider.metadata field, so no default-metadata initializer is registered.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1beta1.DreamKind)

	opts := []managed.ReconcilerOption{
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithManagementPolicies(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1beta1.Dream{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1beta1.DreamGroupVersionKind),
			opts...,
		))
}

// SetupGated registers the Dream controller to start only once the Dream CRD
// is established.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(err)
		}
	}, v1beta1.DreamGroupVersionKind)
	return nil
}

// connector builds an ExternalClient for each reconcile.
type connector struct {
	kube client.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	dr, ok := mg.(*v1beta1.Dream)
	if !ok {
		return nil, xperrors.New(errNotDream)
	}

	cl, err := clients.NewClient(ctx, c.kube, dr)
	if err != nil {
		return nil, xperrors.Wrap(err, errNewClient)
	}

	return &external{client: cl}, nil
}

// external implements managed.ExternalClient for Anthropic Dreams.
type external struct {
	client *anthropic.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	dr, ok := mg.(*v1beta1.Dream)
	if !ok {
		return managed.ExternalObservation{}, xperrors.New(errNotDream)
	}

	// Crossplane seeds external-name with the k8s object name before Create runs.
	// Some Anthropic APIs return 400 (not 404) for non-prefixed IDs, so detect
	// "not yet created" by comparing against the k8s name rather than checking empty.
	dreamID := meta.GetExternalName(dr)
	if dreamID == "" || dreamID == dr.GetName() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	resp, err := e.client.Beta.Dreams.Get(ctx, dreamID, anthropic.BetaDreamGetParams{})
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, xperrors.Wrap(err, errObserve)
	}

	// Archived dreams are treated as deleted — Crossplane will re-create them.
	if !resp.ArchivedAt.IsZero() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	dr.FromAnthropicObservation(*resp)

	dr.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true, // immutable: no update endpoint, never drifts.
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	dr, ok := mg.(*v1beta1.Dream)
	if !ok {
		return managed.ExternalCreation{}, xperrors.New(errNotDream)
	}

	params := dr.ToAnthropicNew()
	resp, err := e.client.Beta.Dreams.New(ctx, params)
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}

	// Store ID in external-name (primary key) and mirror it in AtProvider
	// so cross-resource references can extract it via ComputedFieldExtractor("id").
	meta.SetExternalName(dr, resp.ID)
	dr.Status.AtProvider.ID = &resp.ID

	return managed.ExternalCreation{}, nil
}

// Update is a no-op. A Dream is immutable after creation: the Anthropic API
// exposes no update endpoint, and Observe always reports the resource up to
// date, so this method is never expected to be called.
func (e *external) Update(_ context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	if _, ok := mg.(*v1beta1.Dream); !ok {
		return managed.ExternalUpdate{}, xperrors.New(errNotDream)
	}
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	dr, ok := mg.(*v1beta1.Dream)
	if !ok {
		return managed.ExternalDelete{}, xperrors.New(errNotDream)
	}

	dreamID := meta.GetExternalName(dr)
	if dreamID == "" || dreamID == dr.GetName() {
		return managed.ExternalDelete{}, nil
	}

	// Dreams support Archive only (no hard Delete).
	if _, err := e.client.Beta.Dreams.Archive(ctx, dreamID, anthropic.BetaDreamArchiveParams{}); err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, xperrors.Wrap(err, errDelete)
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(_ context.Context) error { return nil }

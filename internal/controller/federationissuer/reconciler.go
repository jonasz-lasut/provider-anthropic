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

// Package federationissuer implements the Crossplane managed reconciler for the
// Anthropic Admin API FederationIssuer resource.
package federationissuer

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

	v1beta1 "github.com/jonasz-lasut/provider-anthropic/apis/organization/v1beta1"
	"github.com/jonasz-lasut/provider-anthropic/internal/clients"
)

const (
	errNotFederationIssuer  = "managed resource is not a FederationIssuer"
	errNewClient = "cannot build Anthropic client"
	errObserve   = "cannot observe FederationIssuer"
	errCreate    = "cannot create FederationIssuer"
	errUpdate    = "cannot update FederationIssuer"
	errDelete    = "cannot archive FederationIssuer"
)

// Setup adds a controller for FederationIssuer to the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1beta1.FederationIssuerKind)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1beta1.FederationIssuer{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1beta1.FederationIssuerGroupVersionKind),
			managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
			managed.WithLogger(o.Logger.WithValues("controller", name)),
			managed.WithPollInterval(o.PollInterval),
			managed.WithManagementPolicies(),
		))
}

// SetupGated registers the FederationIssuer controller to start only once the
// FederationIssuer CRD is established.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(err)
		}
	}, v1beta1.FederationIssuerGroupVersionKind)
	return nil
}

// connector builds an ExternalClient for each reconcile.
type connector struct {
	kube client.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	fi, ok := mg.(*v1beta1.FederationIssuer)
	if !ok {
		return nil, xperrors.New(errNotFederationIssuer)
	}

	cl, err := clients.NewClient(ctx, c.kube, fi)
	if err != nil {
		return nil, xperrors.Wrap(err, errNewClient)
	}

	return &external{client: cl}, nil
}

// external implements managed.ExternalClient for Anthropic FederationIssuers.
type external struct {
	client *anthropic.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	fi, ok := mg.(*v1beta1.FederationIssuer)
	if !ok {
		return managed.ExternalObservation{}, xperrors.New(errNotFederationIssuer)
	}

	// Crossplane seeds external-name with the k8s object name before Create runs.
	id := meta.GetExternalName(fi)
	if id == "" || id == fi.GetName() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	resp, err := e.client.Beta.Organization.Federation.Issuers.Get(ctx, id, anthropic.BetaOrganizationFederationIssuerGetParams{})
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, xperrors.Wrap(err, errObserve)
	}

	// Archived objects are treated as deleted; Crossplane will re-create them.
	if !resp.ArchivedAt.IsZero() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	fi.FromAnthropicObservation(*resp)

	fi.SetConditions(xpv2.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isUpToDate(fi),
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	fi, ok := mg.(*v1beta1.FederationIssuer)
	if !ok {
		return managed.ExternalCreation{}, xperrors.New(errNotFederationIssuer)
	}

	resp, err := e.client.Beta.Organization.Federation.Issuers.New(ctx, fi.ToAnthropicNew())
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}

	meta.SetExternalName(fi, resp.ID)
	fi.Status.AtProvider.ID = &resp.ID

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	fi, ok := mg.(*v1beta1.FederationIssuer)
	if !ok {
		return managed.ExternalUpdate{}, xperrors.New(errNotFederationIssuer)
	}

	id := meta.GetExternalName(fi)
	if id == "" || id == fi.GetName() {
		return managed.ExternalUpdate{}, xperrors.New("external name not yet set; skipping update")
	}

	if _, err := e.client.Beta.Organization.Federation.Issuers.Update(ctx, id, fi.ToAnthropicUpdate()); err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	fi, ok := mg.(*v1beta1.FederationIssuer)
	if !ok {
		return managed.ExternalDelete{}, xperrors.New(errNotFederationIssuer)
	}

	id := meta.GetExternalName(fi)
	if id == "" || id == fi.GetName() {
		return managed.ExternalDelete{}, nil
	}

	// The Admin API has no delete for this kind; archiving is permanent.
	if _, err := e.client.Beta.Organization.Federation.Issuers.Archive(ctx, id, anthropic.BetaOrganizationFederationIssuerArchiveParams{}); err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, xperrors.Wrap(err, errDelete)
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(_ context.Context) error { return nil }

// isUpToDate performs a structured diff between spec.forProvider and
// status.atProvider, skipping nil ForProvider fields and ForProvider-only
// fields that have no AtProvider counterpart.
func isUpToDate(fi *v1beta1.FederationIssuer) bool {
	fpRaw, err := json.Marshal(fi.Spec.ForProvider)
	if err != nil {
		return true
	}
	apRaw, err := json.Marshal(fi.Status.AtProvider)
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

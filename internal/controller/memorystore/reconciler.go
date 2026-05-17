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

// Package memorystore implements the Crossplane managed reconciler for the
// Anthropic MemoryStore beta API.
package memorystore

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
	errNotMemoryStore = "managed resource is not a MemoryStore"
	errNewClient      = "cannot build Anthropic client"
	errObserve        = "cannot observe MemoryStore"
	errCreate         = "cannot create MemoryStore"
	errUpdate         = "cannot update MemoryStore"
	errDelete         = "cannot delete/archive MemoryStore"
)

// Setup adds a controller for MemoryStore to the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(betav1alpha1.MemoryStoreKind)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&betav1alpha1.MemoryStore{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(betav1alpha1.MemoryStoreGroupVersionKind),
			managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
			managed.WithLogger(o.Logger.WithValues("controller", name)),
			managed.WithPollInterval(o.PollInterval),
			managed.WithManagementPolicies(),
		))
}

// SetupGated registers the MemoryStore controller to start only once the
// MemoryStore CRD is established.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(err)
		}
	}, betav1alpha1.MemoryStoreGroupVersionKind)
	return nil
}

// connector builds an ExternalClient for each reconcile.
type connector struct {
	kube client.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	ms, ok := mg.(*betav1alpha1.MemoryStore)
	if !ok {
		return nil, xperrors.New(errNotMemoryStore)
	}

	cl, err := clients.NewClient(ctx, c.kube, ms)
	if err != nil {
		return nil, xperrors.Wrap(err, errNewClient)
	}

	return &external{client: cl}, nil
}

// external implements managed.ExternalClient for Anthropic MemoryStores.
type external struct {
	client *anthropic.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	ms, ok := mg.(*betav1alpha1.MemoryStore)
	if !ok {
		return managed.ExternalObservation{}, xperrors.New(errNotMemoryStore)
	}

	msID := meta.GetExternalName(ms)
	if msID == ms.GetName() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	resp, err := e.client.Beta.MemoryStores.Get(ctx, msID, anthropic.BetaMemoryStoreGetParams{})
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, xperrors.Wrap(err, errObserve)
	}

	// Archived stores are treated as deleted — Crossplane will re-create them.
	if !resp.ArchivedAt.IsZero() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	ms.Status.AtProvider.ID = &resp.ID
	createdAt := resp.CreatedAt.Format(time.RFC3339)
	updatedAt := resp.UpdatedAt.Format(time.RFC3339)
	ms.Status.AtProvider.CreatedAt = &createdAt
	ms.Status.AtProvider.UpdatedAt = &updatedAt
	if !resp.ArchivedAt.IsZero() {
		archivedAt := resp.ArchivedAt.Format(time.RFC3339)
		ms.Status.AtProvider.ArchivedAt = &archivedAt
	}

	ms.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isUpToDate(ms, resp),
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	ms, ok := mg.(*betav1alpha1.MemoryStore)
	if !ok {
		return managed.ExternalCreation{}, xperrors.New(errNotMemoryStore)
	}

	params := buildNewParams(ms.Spec.ForProvider)
	resp, err := e.client.Beta.MemoryStores.New(ctx, params)
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}

	meta.SetExternalName(ms, resp.ID)
	ms.Status.AtProvider.ID = &resp.ID

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	ms, ok := mg.(*betav1alpha1.MemoryStore)
	if !ok {
		return managed.ExternalUpdate{}, xperrors.New(errNotMemoryStore)
	}

	msID := meta.GetExternalName(ms)
	if msID == ms.GetName() {
		return managed.ExternalUpdate{}, xperrors.New("external name not yet set; skipping update")
	}

	params := buildUpdateParams(ms.Spec.ForProvider)
	if _, err := e.client.Beta.MemoryStores.Update(ctx, msID, params); err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	ms, ok := mg.(*betav1alpha1.MemoryStore)
	if !ok {
		return managed.ExternalDelete{}, xperrors.New(errNotMemoryStore)
	}

	msID := meta.GetExternalName(ms)
	if msID == ms.GetName() {
		return managed.ExternalDelete{}, nil
	}

	policy := betav1alpha1.DeletionPolicyArchive
	if ms.Spec.ForProvider.AnthropicDeletionPolicy != nil {
		policy = *ms.Spec.ForProvider.AnthropicDeletionPolicy
	}

	var err error
	if policy == betav1alpha1.DeletionPolicyDelete {
		_, err = e.client.Beta.MemoryStores.Delete(ctx, msID, anthropic.BetaMemoryStoreDeleteParams{})
	} else {
		_, err = e.client.Beta.MemoryStores.Archive(ctx, msID, anthropic.BetaMemoryStoreArchiveParams{})
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
func buildNewParams(p betav1alpha1.MemoryStoreParameters) anthropic.BetaMemoryStoreNewParams {
	params := anthropic.BetaMemoryStoreNewParams{}
	if p.Name != nil {
		params.Name = *p.Name
	}
	if p.Description != nil {
		params.Description = anthropic.String(*p.Description)
	}
	if p.Metadata != nil {
		params.Metadata = p.Metadata
	}
	return params
}

// buildUpdateParams converts ForProvider into the SDK update params.
func buildUpdateParams(p betav1alpha1.MemoryStoreParameters) anthropic.BetaMemoryStoreUpdateParams {
	params := anthropic.BetaMemoryStoreUpdateParams{}
	if p.Name != nil {
		params.Name = anthropic.String(*p.Name)
	}
	if p.Description != nil {
		params.Description = anthropic.String(*p.Description)
	}
	if p.Metadata != nil {
		params.Metadata = p.Metadata
	}
	return params
}

// isUpToDate compares the desired state with the observed memory store.
func isUpToDate(ms *betav1alpha1.MemoryStore, resp *anthropic.BetaManagedAgentsMemoryStore) bool {
	p := ms.Spec.ForProvider

	if p.Name != nil && *p.Name != resp.Name {
		return false
	}
	if p.Description != nil && *p.Description != resp.Description {
		return false
	}

	if len(p.Metadata) != len(resp.Metadata) {
		return false
	}
	for k, v := range p.Metadata {
		if resp.Metadata[k] != v {
			return false
		}
	}

	return true
}

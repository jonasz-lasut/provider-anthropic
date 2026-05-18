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

// Package memorystorememory implements the Crossplane managed reconciler for
// the Anthropic MemoryStoreMemory beta API.
package memorystorememory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	errNotMemoryStoreMemory = "managed resource is not a MemoryStoreMemory"
	errNewClient            = "cannot build Anthropic client"
	errObserve              = "cannot observe MemoryStoreMemory"
	errCreate               = "cannot create MemoryStoreMemory"
	errUpdate               = "cannot update MemoryStoreMemory"
	errDelete               = "cannot delete MemoryStoreMemory"
	errMissingStore         = "spec.forProvider.memoryStoreId not resolved"
)

// Setup adds a controller for MemoryStoreMemory to the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(betav1alpha1.MemoryStoreMemoryKind)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&betav1alpha1.MemoryStoreMemory{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(betav1alpha1.MemoryStoreMemoryGroupVersionKind),
			managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
			managed.WithLogger(o.Logger.WithValues("controller", name)),
			managed.WithPollInterval(o.PollInterval),
			managed.WithManagementPolicies(),
		))
}

// SetupGated registers the MemoryStoreMemory controller to start only once
// the MemoryStoreMemory CRD is established.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(err)
		}
	}, betav1alpha1.MemoryStoreMemoryGroupVersionKind)
	return nil
}

// connector builds an ExternalClient for each reconcile.
type connector struct {
	kube client.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	m, ok := mg.(*betav1alpha1.MemoryStoreMemory)
	if !ok {
		return nil, xperrors.New(errNotMemoryStoreMemory)
	}

	cl, err := clients.NewClient(ctx, c.kube, m)
	if err != nil {
		return nil, xperrors.Wrap(err, errNewClient)
	}

	return &external{client: cl, kube: c.kube}, nil
}

// external implements managed.ExternalClient for Anthropic MemoryStoreMemories.
type external struct {
	client *anthropic.Client
	kube   client.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	m, ok := mg.(*betav1alpha1.MemoryStoreMemory)
	if !ok {
		return managed.ExternalObservation{}, xperrors.New(errNotMemoryStoreMemory)
	}

	memID := meta.GetExternalName(m)
	if memID == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	if m.Spec.ForProvider.MemoryStoreID == nil || *m.Spec.ForProvider.MemoryStoreID == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	// view=full so we can compare stored content against the spec.
	resp, err := e.client.Beta.MemoryStores.Memories.Get(ctx, memID, anthropic.BetaMemoryStoreMemoryGetParams{
		MemoryStoreID: *m.Spec.ForProvider.MemoryStoreID,
		View:          anthropic.BetaManagedAgentsMemoryViewFull,
	})
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, xperrors.Wrap(err, errObserve)
	}

	m.Status.AtProvider.ID = &resp.ID
	m.Status.AtProvider.MemoryStoreID = &resp.MemoryStoreID
	m.Status.AtProvider.MemoryVersionID = &resp.MemoryVersionID
	m.Status.AtProvider.ContentSha256 = &resp.ContentSha256
	m.Status.AtProvider.ContentSizeBytes = &resp.ContentSizeBytes
	createdAt := resp.CreatedAt.Format(time.RFC3339)
	updatedAt := resp.UpdatedAt.Format(time.RFC3339)
	m.Status.AtProvider.CreatedAt = &createdAt
	m.Status.AtProvider.UpdatedAt = &updatedAt

	m.SetConditions(xpv1.Available())

	desired, err := clients.ResolveLocalSecretKey(ctx, e.kube, m.Spec.ForProvider.ContentSecretRef, m.GetNamespace())
	if err != nil {
		return managed.ExternalObservation{}, xperrors.Wrap(err, errObserve)
	}
	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isUpToDate(m, resp, desired),
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	m, ok := mg.(*betav1alpha1.MemoryStoreMemory)
	if !ok {
		return managed.ExternalCreation{}, xperrors.New(errNotMemoryStoreMemory)
	}

	if m.Spec.ForProvider.MemoryStoreID == nil || *m.Spec.ForProvider.MemoryStoreID == "" {
		return managed.ExternalCreation{}, xperrors.New(errMissingStore)
	}

	params := anthropic.BetaMemoryStoreMemoryNewParams{}
	if m.Spec.ForProvider.Path != nil {
		params.Path = *m.Spec.ForProvider.Path
	}
	content, err := clients.ResolveLocalSecretKey(ctx, e.kube, m.Spec.ForProvider.ContentSecretRef, m.GetNamespace())
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}
	if content != "" {
		params.Content = anthropic.String(content)
	}
	resp, err := e.client.Beta.MemoryStores.Memories.New(ctx, *m.Spec.ForProvider.MemoryStoreID, params)
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}

	meta.SetExternalName(m, resp.ID)
	m.Status.AtProvider.ID = &resp.ID
	m.Status.AtProvider.MemoryStoreID = &resp.MemoryStoreID

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	m, ok := mg.(*betav1alpha1.MemoryStoreMemory)
	if !ok {
		return managed.ExternalUpdate{}, xperrors.New(errNotMemoryStoreMemory)
	}

	memID := meta.GetExternalName(m)
	if memID == "" {
		return managed.ExternalUpdate{}, xperrors.New("external name not yet set; skipping update")
	}

	if m.Spec.ForProvider.MemoryStoreID == nil || *m.Spec.ForProvider.MemoryStoreID == "" {
		return managed.ExternalUpdate{}, xperrors.New(errMissingStore)
	}

	params := anthropic.BetaMemoryStoreMemoryUpdateParams{
		MemoryStoreID: *m.Spec.ForProvider.MemoryStoreID,
	}
	if m.Spec.ForProvider.Path != nil {
		params.Path = anthropic.String(*m.Spec.ForProvider.Path)
	}
	content, err := clients.ResolveLocalSecretKey(ctx, e.kube, m.Spec.ForProvider.ContentSecretRef, m.GetNamespace())
	if err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}
	if content != "" {
		params.Content = anthropic.String(content)
	}
	if _, err := e.client.Beta.MemoryStores.Memories.Update(ctx, memID, params); err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	m, ok := mg.(*betav1alpha1.MemoryStoreMemory)
	if !ok {
		return managed.ExternalDelete{}, xperrors.New(errNotMemoryStoreMemory)
	}

	memID := meta.GetExternalName(m)
	if memID == "" {
		return managed.ExternalDelete{}, nil
	}

	if m.Spec.ForProvider.MemoryStoreID == nil || *m.Spec.ForProvider.MemoryStoreID == "" {
		// Without a resolved parent we cannot target the API; treat as no-op.
		return managed.ExternalDelete{}, nil
	}

	_, err := e.client.Beta.MemoryStores.Memories.Delete(ctx, memID, anthropic.BetaMemoryStoreMemoryDeleteParams{
		MemoryStoreID: *m.Spec.ForProvider.MemoryStoreID,
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

// isUpToDate compares the desired state with the observed memory.
// resolvedContent is the bytes currently in the referenced Secret; passing
// an empty string means "no Secret content provided" and the field is not
// diffed.
func isUpToDate(m *betav1alpha1.MemoryStoreMemory, resp *anthropic.BetaManagedAgentsMemory, resolvedContent string) bool {
	p := m.Spec.ForProvider

	if p.Path != nil && *p.Path != resp.Path {
		return false
	}
	if resolvedContent != "" {
		sum := sha256.Sum256([]byte(resolvedContent))
		if hex.EncodeToString(sum[:]) != resp.ContentSha256 {
			return false
		}
	}
	return true
}

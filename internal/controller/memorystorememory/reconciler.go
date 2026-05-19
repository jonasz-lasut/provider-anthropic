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

// Package memorystorememory implements the Crossplane managed reconciler for
// the Anthropic MemoryStoreMemory beta API.
package memorystorememory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

	betav1alpha1 "github.com/jonasz-lasut/provider-anthropic/apis/beta/v1alpha1"
	"github.com/jonasz-lasut/provider-anthropic/internal/clients"
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

	// Crossplane seeds external-name with the k8s object name before Create runs.
	// The Memories API returns 400 (not 404) for non-mem_... IDs, so we detect
	// "not yet created" by comparing against the k8s name rather than checking empty.
	memID := meta.GetExternalName(m)
	if memID == "" || memID == m.GetName() {
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

	m.FromAnthropicObservation(*resp)

	m.SetConditions(xpv1.Available())

	desired, err := clients.ResolveLocalSecretKey(ctx, e.kube, m.Spec.ForProvider.ContentSecretRef, m.GetNamespace())
	if err != nil {
		return managed.ExternalObservation{}, xperrors.Wrap(err, errObserve)
	}
	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isUpToDate(m, desired),
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

	content, err := clients.ResolveLocalSecretKey(ctx, e.kube, m.Spec.ForProvider.ContentSecretRef, m.GetNamespace())
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}
	params := m.ToAnthropicNew(&betav1alpha1.MemoryStoreMemoryConversionContext{Content: content})
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
	if memID == "" || memID == m.GetName() {
		return managed.ExternalUpdate{}, xperrors.New("external name not yet set; skipping update")
	}

	if m.Spec.ForProvider.MemoryStoreID == nil || *m.Spec.ForProvider.MemoryStoreID == "" {
		return managed.ExternalUpdate{}, xperrors.New(errMissingStore)
	}

	content, err := clients.ResolveLocalSecretKey(ctx, e.kube, m.Spec.ForProvider.ContentSecretRef, m.GetNamespace())
	if err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}
	params := m.ToAnthropicUpdate(&betav1alpha1.MemoryStoreMemoryConversionContext{Content: content})
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
	if memID == "" || memID == m.GetName() {
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

// isUpToDate performs a structured diff between spec.forProvider and
// status.atProvider for non-secret fields, then separately checks content
// drift by comparing the resolved secret's SHA-256 against the observed hash.
func isUpToDate(m *betav1alpha1.MemoryStoreMemory, resolvedContent string) bool {
	fpRaw, err := json.Marshal(m.Spec.ForProvider)
	if err != nil {
		return true
	}
	apRaw, err := json.Marshal(m.Status.AtProvider)
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
	if !clients.IsSubsetEqual(fp, ap) {
		return false
	}

	// SecretRef drift: compare resolved content SHA-256 against observed hash.
	if resolvedContent != "" {
		sum := sha256.Sum256([]byte(resolvedContent))
		if m.Status.AtProvider.ContentSha256 == nil || hex.EncodeToString(sum[:]) != *m.Status.AtProvider.ContentSha256 {
			return false
		}
	}
	return true
}

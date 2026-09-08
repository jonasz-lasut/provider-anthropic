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

// Package externalkey implements the Crossplane managed reconciler for the
// Anthropic Admin API ExternalKey (CMEK) resource.
package externalkey

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
	errNotExternalKey = "managed resource is not an ExternalKey"
	errNewClient      = "cannot build Anthropic client"
	errObserve        = "cannot observe ExternalKey"
	errCreate         = "cannot create ExternalKey"
	errUpdate         = "cannot update ExternalKey"
	errDelete         = "cannot delete ExternalKey"
)

// Setup adds a controller for ExternalKey to the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1beta1.ExternalKeyKind)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1beta1.ExternalKey{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1beta1.ExternalKeyGroupVersionKind),
			managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
			managed.WithLogger(o.Logger.WithValues("controller", name)),
			managed.WithPollInterval(o.PollInterval),
			managed.WithManagementPolicies(),
		))
}

// SetupGated registers the ExternalKey controller to start only once the
// ExternalKey CRD is established.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(err)
		}
	}, v1beta1.ExternalKeyGroupVersionKind)
	return nil
}

// connector builds an ExternalClient for each reconcile.
type connector struct {
	kube client.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	k, ok := mg.(*v1beta1.ExternalKey)
	if !ok {
		return nil, xperrors.New(errNotExternalKey)
	}

	cl, err := clients.NewClient(ctx, c.kube, k)
	if err != nil {
		return nil, xperrors.Wrap(err, errNewClient)
	}

	return &external{client: cl}, nil
}

// external implements managed.ExternalClient for Anthropic ExternalKeys.
type external struct {
	client *anthropic.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	k, ok := mg.(*v1beta1.ExternalKey)
	if !ok {
		return managed.ExternalObservation{}, xperrors.New(errNotExternalKey)
	}

	// Crossplane seeds external-name with the k8s object name before Create runs.
	kID := meta.GetExternalName(k)
	if kID == "" || kID == k.GetName() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	resp, err := e.client.Beta.Organization.ExternalKeys.Get(ctx, kID)
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, xperrors.Wrap(err, errObserve)
	}

	k.FromAnthropicObservation(*resp)

	k.SetConditions(xpv2.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isUpToDate(k),
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	k, ok := mg.(*v1beta1.ExternalKey)
	if !ok {
		return managed.ExternalCreation{}, xperrors.New(errNotExternalKey)
	}

	resp, err := e.client.Beta.Organization.ExternalKeys.New(ctx, k.ToAnthropicNew())
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}

	meta.SetExternalName(k, resp.ID)
	k.Status.AtProvider.ID = &resp.ID

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	k, ok := mg.(*v1beta1.ExternalKey)
	if !ok {
		return managed.ExternalUpdate{}, xperrors.New(errNotExternalKey)
	}

	kID := meta.GetExternalName(k)
	if kID == "" || kID == k.GetName() {
		return managed.ExternalUpdate{}, xperrors.New("external name not yet set; skipping update")
	}

	if _, err := e.client.Beta.Organization.ExternalKeys.Update(ctx, kID, k.ToAnthropicUpdate()); err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	k, ok := mg.(*v1beta1.ExternalKey)
	if !ok {
		return managed.ExternalDelete{}, xperrors.New(errNotExternalKey)
	}

	kID := meta.GetExternalName(k)
	if kID == "" || kID == k.GetName() {
		return managed.ExternalDelete{}, nil
	}

	if _, err := e.client.Beta.Organization.ExternalKeys.Delete(ctx, kID); err != nil {
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
// status.atProvider, skipping nil ForProvider fields.
func isUpToDate(k *v1beta1.ExternalKey) bool {
	fpRaw, err := json.Marshal(k.Spec.ForProvider)
	if err != nil {
		return true
	}
	apRaw, err := json.Marshal(k.Status.AtProvider)
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

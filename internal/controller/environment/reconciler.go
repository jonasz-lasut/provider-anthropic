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

// Package environment implements the Crossplane managed reconciler for the
// Anthropic Environments beta API.
package environment

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
	errNotEnvironment = "managed resource is not an Environment"
	errNewClient      = "cannot build Anthropic client"
	errObserve        = "cannot observe Environment"
	errCreate         = "cannot create Environment"
	errUpdate         = "cannot update Environment"
	errDelete         = "cannot delete/archive Environment"
)

// Setup adds a controller for Environment to the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options, skipDefaultMetadata bool) error {
	name := managed.ControllerName(v1beta1.EnvironmentKind)

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
		For(&v1beta1.Environment{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1beta1.EnvironmentGroupVersionKind),
			opts...,
		))
}

// SetupGated registers the Environment controller to start only once the
// Environment CRD is established.
func SetupGated(mgr ctrl.Manager, o controller.Options, skipDefaultMetadata bool) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o, skipDefaultMetadata); err != nil {
			panic(err)
		}
	}, v1beta1.EnvironmentGroupVersionKind)
	return nil
}

// connector builds an ExternalClient for each reconcile.
type connector struct {
	kube client.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	env, ok := mg.(*v1beta1.Environment)
	if !ok {
		return nil, xperrors.New(errNotEnvironment)
	}

	cl, err := clients.NewClient(ctx, c.kube, env)
	if err != nil {
		return nil, xperrors.Wrap(err, errNewClient)
	}

	return &external{client: cl}, nil
}

// external implements managed.ExternalClient for Anthropic Environments.
type external struct {
	client *anthropic.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	env, ok := mg.(*v1beta1.Environment)
	if !ok {
		return managed.ExternalObservation{}, xperrors.New(errNotEnvironment)
	}

	// Crossplane seeds external-name with the k8s object name before Create runs.
	// Some Anthropic APIs return 400 (not 404) for non-prefixed IDs, so detect
	// "not yet created" by comparing against the k8s name rather than checking empty.
	envID := meta.GetExternalName(env)
	if envID == "" || envID == env.GetName() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	resp, err := e.client.Beta.Environments.Get(ctx, envID, anthropic.BetaEnvironmentGetParams{})
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, xperrors.Wrap(err, errObserve)
	}

	// Archived environments are treated as deleted — Crossplane will re-create them.
	if resp.ArchivedAt != "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	env.FromAnthropicObservation(*resp)

	env.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isUpToDate(env),
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	env, ok := mg.(*v1beta1.Environment)
	if !ok {
		return managed.ExternalCreation{}, xperrors.New(errNotEnvironment)
	}

	params := env.ToAnthropicNew()
	resp, err := e.client.Beta.Environments.New(ctx, params)
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}

	// Store ID in external-name (primary key) and mirror it in AtProvider
	// so cross-resource references can extract it via ComputedFieldExtractor("id").
	meta.SetExternalName(env, resp.ID)
	env.Status.AtProvider.ID = &resp.ID

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	env, ok := mg.(*v1beta1.Environment)
	if !ok {
		return managed.ExternalUpdate{}, xperrors.New(errNotEnvironment)
	}

	envID := meta.GetExternalName(env)
	if envID == "" || envID == env.GetName() {
		return managed.ExternalUpdate{}, xperrors.New("external name not yet set; skipping update")
	}

	params := env.ToAnthropicUpdate()
	if _, err := e.client.Beta.Environments.Update(ctx, envID, params); err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	env, ok := mg.(*v1beta1.Environment)
	if !ok {
		return managed.ExternalDelete{}, xperrors.New(errNotEnvironment)
	}

	envID := meta.GetExternalName(env)
	if envID == "" || envID == env.GetName() {
		return managed.ExternalDelete{}, nil
	}

	policy := v1beta1.DeletionPolicyArchive
	if env.Spec.ForProvider.AnthropicDeletionPolicy != nil {
		policy = *env.Spec.ForProvider.AnthropicDeletionPolicy
	}

	var err error
	if policy == v1beta1.DeletionPolicyDelete {
		_, err = e.client.Beta.Environments.Delete(ctx, envID, anthropic.BetaEnvironmentDeleteParams{})
	} else {
		_, err = e.client.Beta.Environments.Archive(ctx, envID, anthropic.BetaEnvironmentArchiveParams{})
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

// isUpToDate performs a structured diff between spec.forProvider and
// status.atProvider. Nil ForProvider fields and ForProvider-only fields
// (AnthropicDeletionPolicy) are skipped automatically.
func isUpToDate(env *v1beta1.Environment) bool {
	fpRaw, err := json.Marshal(env.Spec.ForProvider)
	if err != nil {
		return true
	}
	apRaw, err := json.Marshal(env.Status.AtProvider)
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

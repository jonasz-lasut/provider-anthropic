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

// Package deployment implements the Crossplane managed reconciler for the
// Anthropic Deployments beta API.
package deployment

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

	betav1alpha1 "github.com/jonasz-lasut/provider-anthropic/apis/beta/v1alpha1"
	"github.com/jonasz-lasut/provider-anthropic/internal/clients"
	"github.com/jonasz-lasut/provider-anthropic/internal/initializer"
)

const (
	errNotDeployment = "managed resource is not a Deployment"
	errNewClient     = "cannot build Anthropic client"
	errObserve       = "cannot observe Deployment"
	errCreate        = "cannot create Deployment"
	errUpdate        = "cannot update Deployment"
	errDelete        = "cannot delete/archive Deployment"
)

// Setup adds a controller for Deployment to the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options, skipDefaultMetadata bool) error {
	name := managed.ControllerName(betav1alpha1.DeploymentKind)

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
		For(&betav1alpha1.Deployment{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(betav1alpha1.DeploymentGroupVersionKind),
			opts...,
		))
}

// SetupGated registers the Deployment controller to start only once the
// Deployment CRD is established.
func SetupGated(mgr ctrl.Manager, o controller.Options, skipDefaultMetadata bool) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o, skipDefaultMetadata); err != nil {
			panic(err)
		}
	}, betav1alpha1.DeploymentGroupVersionKind)
	return nil
}

// connector builds an ExternalClient for each reconcile.
type connector struct {
	kube client.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	dep, ok := mg.(*betav1alpha1.Deployment)
	if !ok {
		return nil, xperrors.New(errNotDeployment)
	}

	cl, err := clients.NewClient(ctx, c.kube, dep)
	if err != nil {
		return nil, xperrors.Wrap(err, errNewClient)
	}

	return &external{client: cl, kube: c.kube}, nil
}

// external implements managed.ExternalClient for Anthropic Deployments.
type external struct {
	client *anthropic.Client
	kube   client.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	dep, ok := mg.(*betav1alpha1.Deployment)
	if !ok {
		return managed.ExternalObservation{}, xperrors.New(errNotDeployment)
	}

	// Crossplane seeds external-name with the k8s object name before Create runs.
	// Some Anthropic APIs return 400 (not 404) for non-prefixed IDs, so detect
	// "not yet created" by comparing against the k8s name rather than checking empty.
	depID := meta.GetExternalName(dep)
	if depID == "" || depID == dep.GetName() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	resp, err := e.client.Beta.Deployments.Get(ctx, depID, anthropic.BetaDeploymentGetParams{})
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, xperrors.Wrap(err, errObserve)
	}

	// Archived deployments are treated as deleted — Crossplane will re-create them.
	if !resp.ArchivedAt.IsZero() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	dep.FromAnthropicObservation(*resp)

	dep.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isUpToDate(dep),
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	dep, ok := mg.(*betav1alpha1.Deployment)
	if !ok {
		return managed.ExternalCreation{}, xperrors.New(errNotDeployment)
	}

	cctx, err := resolveDeploymentContext(ctx, e.kube, dep)
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}

	params := dep.ToAnthropicNew(cctx)
	resp, err := e.client.Beta.Deployments.New(ctx, params)
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}

	// Store ID in external-name (primary key) and mirror it in AtProvider
	// so cross-resource references can extract it via ComputedFieldExtractor("id").
	meta.SetExternalName(dep, resp.ID)
	dep.Status.AtProvider.ID = &resp.ID

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	dep, ok := mg.(*betav1alpha1.Deployment)
	if !ok {
		return managed.ExternalUpdate{}, xperrors.New(errNotDeployment)
	}

	depID := meta.GetExternalName(dep)
	if depID == "" || depID == dep.GetName() {
		return managed.ExternalUpdate{}, xperrors.New("external name not yet set; skipping update")
	}

	cctx, err := resolveDeploymentContext(ctx, e.kube, dep)
	if err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}

	params := dep.ToAnthropicUpdate(cctx)
	if _, err := e.client.Beta.Deployments.Update(ctx, depID, params); err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}

	if err := e.reconcilePause(ctx, dep, depID); err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}

	return managed.ExternalUpdate{}, nil
}

// reconcilePause converges the deployment's observed pause state to the desired
// spec.forProvider.paused via the imperative Pause/Unpause endpoints. A nil
// desired value means the user does not manage pause state, so it is left as-is.
func (e *external) reconcilePause(ctx context.Context, dep *betav1alpha1.Deployment, depID string) error {
	if dep.Spec.ForProvider.Paused == nil {
		return nil
	}
	desired := *dep.Spec.ForProvider.Paused
	observed := dep.Status.AtProvider.Paused != nil && *dep.Status.AtProvider.Paused
	if desired == observed {
		return nil
	}
	var err error
	if desired {
		_, err = e.client.Beta.Deployments.Pause(ctx, depID, anthropic.BetaDeploymentPauseParams{})
	} else {
		_, err = e.client.Beta.Deployments.Unpause(ctx, depID, anthropic.BetaDeploymentUnpauseParams{})
	}
	return err
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	dep, ok := mg.(*betav1alpha1.Deployment)
	if !ok {
		return managed.ExternalDelete{}, xperrors.New(errNotDeployment)
	}

	depID := meta.GetExternalName(dep)
	if depID == "" || depID == dep.GetName() {
		return managed.ExternalDelete{}, nil
	}

	// The Deployments service exposes Archive only (no Delete).
	if _, err := e.client.Beta.Deployments.Archive(ctx, depID, anthropic.BetaDeploymentArchiveParams{}); err != nil {
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
// status.atProvider. initialEvents and resources are absent from AtProvider
// (see DeploymentObservation doc) and so are not independently drift-detected;
// they are still sent on every update. Ref/Selector ForProvider-only fields are
// absent from AtProvider and skipped automatically.
func isUpToDate(dep *betav1alpha1.Deployment) bool {
	fpRaw, err := json.Marshal(dep.Spec.ForProvider)
	if err != nil {
		return true
	}
	apRaw, err := json.Marshal(dep.Status.AtProvider)
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

// resolveDeploymentContext pre-resolves the per-resource Kubernetes secrets
// referenced by ForProvider.Resources[*].AuthorizationTokenSecretRef into a
// DeploymentConversionContext. Called from Create and Update.
func resolveDeploymentContext(ctx context.Context, kube client.Client, dep *betav1alpha1.Deployment) (*betav1alpha1.DeploymentConversionContext, error) {
	cctx := &betav1alpha1.DeploymentConversionContext{
		ResourceTokens: make([]string, len(dep.Spec.ForProvider.Resources)),
	}
	for i, res := range dep.Spec.ForProvider.Resources {
		token, err := clients.ResolveLocalSecretKey(ctx, kube, res.AuthorizationTokenSecretRef, dep.GetNamespace())
		if err != nil {
			return nil, err
		}
		cctx.ResourceTokens[i] = token
	}
	return cctx, nil
}

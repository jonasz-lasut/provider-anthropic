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

// Package session implements the Crossplane managed reconciler for the
// Anthropic Sessions beta API.
package session

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
	errNotSession = "managed resource is not a Session"
	errNewClient  = "cannot build Anthropic client"
	errObserve    = "cannot observe Session"
	errCreate     = "cannot create Session"
	errUpdate     = "cannot update Session"
	errDelete     = "cannot delete/archive Session"
)

// Setup adds a controller for Session to the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options, skipDefaultMetadata bool) error {
	name := managed.ControllerName(v1beta1.SessionKind)

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
		For(&v1beta1.Session{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1beta1.SessionGroupVersionKind),
			opts...,
		))
}

// SetupGated registers the Session controller to start only once the
// Session CRD is established.
func SetupGated(mgr ctrl.Manager, o controller.Options, skipDefaultMetadata bool) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o, skipDefaultMetadata); err != nil {
			panic(err)
		}
	}, v1beta1.SessionGroupVersionKind)
	return nil
}

// connector builds an ExternalClient for each reconcile.
type connector struct {
	kube client.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	sess, ok := mg.(*v1beta1.Session)
	if !ok {
		return nil, xperrors.New(errNotSession)
	}

	cl, err := clients.NewClient(ctx, c.kube, sess)
	if err != nil {
		return nil, xperrors.Wrap(err, errNewClient)
	}

	return &external{client: cl, kube: c.kube}, nil
}

// external implements managed.ExternalClient for Anthropic Sessions.
type external struct {
	client *anthropic.Client
	kube   client.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	sess, ok := mg.(*v1beta1.Session)
	if !ok {
		return managed.ExternalObservation{}, xperrors.New(errNotSession)
	}

	// Crossplane seeds external-name with the k8s object name before Create runs.
	// Some Anthropic APIs return 400 (not 404) for non-prefixed IDs, so detect
	// "not yet created" by comparing against the k8s name rather than checking empty.
	sessID := meta.GetExternalName(sess)
	if sessID == "" || sessID == sess.GetName() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	resp, err := e.client.Beta.Sessions.Get(ctx, sessID, anthropic.BetaSessionGetParams{})
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, xperrors.Wrap(err, errObserve)
	}

	// Archived sessions are treated as deleted — Crossplane will re-create them.
	if !resp.ArchivedAt.IsZero() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	sess.FromAnthropicObservation(*resp)

	sess.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isUpToDate(sess),
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	sess, ok := mg.(*v1beta1.Session)
	if !ok {
		return managed.ExternalCreation{}, xperrors.New(errNotSession)
	}

	cctx, err := resolveSessionContext(ctx, e.kube, sess)
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}

	params := sess.ToAnthropicNew(cctx)
	resp, err := e.client.Beta.Sessions.New(ctx, params)
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}

	// Store ID in external-name (primary key) and mirror it in AtProvider
	// so cross-resource references can extract it via ComputedFieldExtractor("id").
	meta.SetExternalName(sess, resp.ID)
	sess.Status.AtProvider.ID = &resp.ID

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	sess, ok := mg.(*v1beta1.Session)
	if !ok {
		return managed.ExternalUpdate{}, xperrors.New(errNotSession)
	}

	sessID := meta.GetExternalName(sess)
	if sessID == "" || sessID == sess.GetName() {
		return managed.ExternalUpdate{}, xperrors.New("external name not yet set; skipping update")
	}

	params := sess.ToAnthropicUpdate()
	if _, err := e.client.Beta.Sessions.Update(ctx, sessID, params); err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	sess, ok := mg.(*v1beta1.Session)
	if !ok {
		return managed.ExternalDelete{}, xperrors.New(errNotSession)
	}

	sessID := meta.GetExternalName(sess)
	if sessID == "" || sessID == sess.GetName() {
		return managed.ExternalDelete{}, nil
	}

	policy := v1beta1.DeletionPolicyArchive
	if sess.Spec.ForProvider.AnthropicDeletionPolicy != nil {
		policy = *sess.Spec.ForProvider.AnthropicDeletionPolicy
	}

	var err error
	if policy == v1beta1.DeletionPolicyDelete {
		_, err = e.client.Beta.Sessions.Delete(ctx, sessID, anthropic.BetaSessionDeleteParams{})
	} else {
		_, err = e.client.Beta.Sessions.Archive(ctx, sessID, anthropic.BetaSessionArchiveParams{})
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
// status.atProvider. Immutable fields (AgentID, EnvironmentID, Resources,
// VaultIDs) are included in the diff but never reported as drift because
// the API ensures they never change after creation. Ref/Selector and
// SecretRef ForProvider-only fields are absent from AtProvider and skipped.
func isUpToDate(sess *v1beta1.Session) bool {
	fpRaw, err := json.Marshal(sess.Spec.ForProvider)
	if err != nil {
		return true
	}
	apRaw, err := json.Marshal(sess.Status.AtProvider)
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

// resolveSessionContext pre-resolves the per-resource Kubernetes secrets
// referenced by ForProvider.Resources[*].AuthorizationTokenSecretRef into a
// SessionConversionContext. Called from Create only; Resources are immutable
// post-creation so Update never re-resolves.
func resolveSessionContext(ctx context.Context, kube client.Client, sess *v1beta1.Session) (*v1beta1.SessionConversionContext, error) {
	cctx := &v1beta1.SessionConversionContext{
		ResourceTokens: make([]string, len(sess.Spec.ForProvider.Resources)),
	}
	for i, res := range sess.Spec.ForProvider.Resources {
		token, err := clients.ResolveLocalSecretKey(ctx, kube, res.AuthorizationTokenSecretRef, sess.GetNamespace())
		if err != nil {
			return nil, err
		}
		cctx.ResourceTokens[i] = token
	}
	return cctx, nil
}

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

// Package agent implements the Crossplane managed reconciler for the Anthropic
// Managed Agents beta API.
package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

	v1beta1 "github.com/jonasz-lasut/provider-anthropic/apis/managedagents/v1beta1"
	"github.com/jonasz-lasut/provider-anthropic/internal/clients"
	"github.com/jonasz-lasut/provider-anthropic/internal/initializer"
)

const (
	errNotAgent  = "managed resource is not an Agent"
	errNewClient = "cannot build Anthropic client"
	errObserve   = "cannot observe Agent"
	errCreate    = "cannot create Agent"
	errUpdate    = "cannot update Agent"
	errDelete    = "cannot archive Agent"
)

// Setup adds a controller for Agent to the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options, skipDefaultMetadata bool) error {
	name := managed.ControllerName(v1beta1.AgentKind)

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
		For(&v1beta1.Agent{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1beta1.AgentGroupVersionKind),
			opts...,
		))
}

// SetupGated registers the Agent controller to start only once the Agent CRD is established.
func SetupGated(mgr ctrl.Manager, o controller.Options, skipDefaultMetadata bool) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o, skipDefaultMetadata); err != nil {
			panic(err)
		}
	}, v1beta1.AgentGroupVersionKind)
	return nil
}

// connector builds an ExternalClient for each reconcile.
type connector struct {
	kube client.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	ag, ok := mg.(*v1beta1.Agent)
	if !ok {
		return nil, xperrors.New(errNotAgent)
	}

	cl, err := clients.NewClient(ctx, c.kube, ag)
	if err != nil {
		return nil, xperrors.Wrap(err, errNewClient)
	}

	return &external{client: cl, kube: c.kube}, nil
}

// external implements managed.ExternalClient for Anthropic Agents.
type external struct {
	client *anthropic.Client
	kube   client.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	ag, ok := mg.(*v1beta1.Agent)
	if !ok {
		return managed.ExternalObservation{}, xperrors.New(errNotAgent)
	}

	// Crossplane seeds external-name with the k8s object name before Create runs.
	// Some Anthropic APIs return 400 (not 404) for non-prefixed IDs, so detect
	// "not yet created" by comparing against the k8s name rather than checking empty.
	agentID := meta.GetExternalName(ag)
	if agentID == "" || agentID == ag.GetName() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	resp, err := e.client.Beta.Agents.Get(ctx, agentID, anthropic.BetaAgentGetParams{})
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, xperrors.Wrap(err, errObserve)
	}

	// Archived agents are treated as deleted — Crossplane will re-create them.
	if !resp.ArchivedAt.IsZero() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	ag.FromAnthropicObservation(*resp)

	ag.SetConditions(xpv2.Available())

	system, err := clients.ResolveLocalSecretKey(ctx, e.kube, ag.Spec.ForProvider.SystemSecretRef, ag.GetNamespace())
	if err != nil {
		return managed.ExternalObservation{}, xperrors.Wrap(err, errObserve)
	}
	convCtx := &v1beta1.AgentConversionContext{System: system}
	return managed.ExternalObservation{
		ResourceExists:    true,
		ResourceUpToDate:  isUpToDate(ag, system),
		ConnectionDetails: convCtx.ToConnectionDetails(),
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	ag, ok := mg.(*v1beta1.Agent)
	if !ok {
		return managed.ExternalCreation{}, xperrors.New(errNotAgent)
	}

	system, err := clients.ResolveLocalSecretKey(ctx, e.kube, ag.Spec.ForProvider.SystemSecretRef, ag.GetNamespace())
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}
	convCtx := &v1beta1.AgentConversionContext{System: system}
	params := ag.ToAnthropicNew(convCtx)
	resp, err := e.client.Beta.Agents.New(ctx, params)
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}

	// Store ID in external-name (primary key for the reconciler) and mirror
	// it in AtProvider so cross-resource references can extract it via
	// ComputedFieldExtractor("id").
	meta.SetExternalName(ag, resp.ID)
	ag.Status.AtProvider.ID = &resp.ID

	return managed.ExternalCreation{ConnectionDetails: convCtx.ToConnectionDetails()}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	ag, ok := mg.(*v1beta1.Agent)
	if !ok {
		return managed.ExternalUpdate{}, xperrors.New(errNotAgent)
	}

	agentID := meta.GetExternalName(ag)
	if agentID == "" || agentID == ag.GetName() {
		return managed.ExternalUpdate{}, xperrors.New("external name not yet set; skipping update")
	}

	if ag.Status.AtProvider.Version == nil {
		return managed.ExternalUpdate{}, xperrors.New("version not yet observed; skipping update")
	}

	system, err := clients.ResolveLocalSecretKey(ctx, e.kube, ag.Spec.ForProvider.SystemSecretRef, ag.GetNamespace())
	if err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}
	convCtx := &v1beta1.AgentConversionContext{System: system}
	params := ag.ToAnthropicUpdate(convCtx)
	if _, err := e.client.Beta.Agents.Update(ctx, agentID, params); err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}

	return managed.ExternalUpdate{ConnectionDetails: convCtx.ToConnectionDetails()}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	ag, ok := mg.(*v1beta1.Agent)
	if !ok {
		return managed.ExternalDelete{}, xperrors.New(errNotAgent)
	}

	agentID := meta.GetExternalName(ag)
	if agentID == "" || agentID == ag.GetName() {
		return managed.ExternalDelete{}, nil
	}

	_, err := e.client.Beta.Agents.Archive(ctx, agentID, anthropic.BetaAgentArchiveParams{})
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
// status.atProvider. Nil ForProvider fields (omitempty → absent from JSON)
// are skipped. ForProvider-only fields (SystemSecretRef, Ref/Selector fields)
// that have no AtProvider counterpart are also skipped automatically.
// The system prompt is compared separately because its ForProvider
// representation is a SecretRef, not a plain string.
func isUpToDate(ag *v1beta1.Agent, system string) bool {
	fpRaw, err := json.Marshal(ag.Spec.ForProvider)
	if err != nil {
		return true
	}
	apRaw, err := json.Marshal(ag.Status.AtProvider)
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

	// SecretRef drift: compare SHA-256 of resolved system prompt against observed hash.
	if system != "" {
		sum := sha256.Sum256([]byte(system))
		want := hex.EncodeToString(sum[:])
		if ag.Status.AtProvider.SystemSha256 == nil || want != *ag.Status.AtProvider.SystemSha256 {
			return false
		}
	}

	return true
}

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

// Package workspacemember implements the Crossplane managed reconciler for
// the Anthropic Admin API WorkspaceMember resource.
package workspacemember

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
	errNotWorkspaceMember = "managed resource is not a WorkspaceMember"
	errNewClient          = "cannot build Anthropic client"
	errObserve            = "cannot observe WorkspaceMember"
	errCreate             = "cannot add WorkspaceMember"
	errUpdate             = "cannot update WorkspaceMember"
	errDelete             = "cannot remove WorkspaceMember"
	errMissingWorkspace   = "spec.forProvider.workspaceId not resolved"
)

// Setup adds a controller for WorkspaceMember to the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1beta1.WorkspaceMemberKind)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1beta1.WorkspaceMember{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1beta1.WorkspaceMemberGroupVersionKind),
			managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
			managed.WithLogger(o.Logger.WithValues("controller", name)),
			managed.WithPollInterval(o.PollInterval),
			managed.WithManagementPolicies(),
		))
}

// SetupGated registers the WorkspaceMember controller to start only once the
// WorkspaceMember CRD is established.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(err)
		}
	}, v1beta1.WorkspaceMemberGroupVersionKind)
	return nil
}

// connector builds an ExternalClient for each reconcile.
type connector struct {
	kube client.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	m, ok := mg.(*v1beta1.WorkspaceMember)
	if !ok {
		return nil, xperrors.New(errNotWorkspaceMember)
	}

	cl, err := clients.NewClient(ctx, c.kube, m)
	if err != nil {
		return nil, xperrors.Wrap(err, errNewClient)
	}

	return &external{client: cl}, nil
}

// external implements managed.ExternalClient for Anthropic WorkspaceMembers.
// A membership has no ID of its own: the external-name annotation stores the
// user ID and every call also carries the parent workspace ID.
type external struct {
	client *anthropic.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	m, ok := mg.(*v1beta1.WorkspaceMember)
	if !ok {
		return managed.ExternalObservation{}, xperrors.New(errNotWorkspaceMember)
	}

	// Crossplane seeds external-name with the k8s object name before Create
	// runs; after Add it holds the user ID.
	userID := meta.GetExternalName(m)
	if userID == "" || userID == m.GetName() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	workspaceID := workspaceIDOf(m)
	if workspaceID == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	resp, err := e.client.Beta.Organization.Workspaces.Members.Get(ctx, userID, anthropic.BetaOrganizationWorkspaceMemberGetParams{
		WorkspaceID: workspaceID,
	})
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		// The members endpoint answers 400 once the parent workspace is
		// archived. A membership in an archived workspace is moot, so report
		// it absent; that also lets a deleting member drop its finalizer.
		if e.workspaceGone(ctx, workspaceID) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, xperrors.Wrap(err, errObserve)
	}

	m.FromAnthropicObservation(*resp)

	m.SetConditions(xpv2.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isUpToDate(m),
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	m, ok := mg.(*v1beta1.WorkspaceMember)
	if !ok {
		return managed.ExternalCreation{}, xperrors.New(errNotWorkspaceMember)
	}

	workspaceID := workspaceIDOf(m)
	if workspaceID == "" {
		return managed.ExternalCreation{}, xperrors.New(errMissingWorkspace)
	}

	resp, err := e.client.Beta.Organization.Workspaces.Members.Add(ctx, workspaceID, m.ToAnthropicNew())
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}

	meta.SetExternalName(m, resp.UserID)
	m.Status.AtProvider.UserID = &resp.UserID
	m.Status.AtProvider.WorkspaceID = &resp.WorkspaceID

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	m, ok := mg.(*v1beta1.WorkspaceMember)
	if !ok {
		return managed.ExternalUpdate{}, xperrors.New(errNotWorkspaceMember)
	}

	userID := meta.GetExternalName(m)
	if userID == "" || userID == m.GetName() {
		return managed.ExternalUpdate{}, xperrors.New("external name not yet set; skipping update")
	}

	if workspaceIDOf(m) == "" {
		return managed.ExternalUpdate{}, xperrors.New(errMissingWorkspace)
	}

	if _, err := e.client.Beta.Organization.Workspaces.Members.Update(ctx, userID, m.ToAnthropicUpdate()); err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	m, ok := mg.(*v1beta1.WorkspaceMember)
	if !ok {
		return managed.ExternalDelete{}, xperrors.New(errNotWorkspaceMember)
	}

	userID := meta.GetExternalName(m)
	if userID == "" || userID == m.GetName() {
		return managed.ExternalDelete{}, nil
	}

	workspaceID := workspaceIDOf(m)
	if workspaceID == "" {
		// Without a resolved parent we cannot target the API; treat as no-op.
		return managed.ExternalDelete{}, nil
	}

	_, err := e.client.Beta.Organization.Workspaces.Members.Remove(ctx, userID, anthropic.BetaOrganizationWorkspaceMemberRemoveParams{
		WorkspaceID: workspaceID,
	})
	if err == nil {
		return managed.ExternalDelete{}, nil
	}
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
		return managed.ExternalDelete{}, nil
	}
	// The parent may already be archived (Crossplane deletes the Workspace and
	// its members concurrently); a membership in an archived workspace is
	// moot, so treat that case as removed.
	if e.workspaceGone(ctx, workspaceID) {
		return managed.ExternalDelete{}, nil
	}
	return managed.ExternalDelete{}, xperrors.Wrap(err, errDelete)
}

func (e *external) Disconnect(_ context.Context) error { return nil }

// workspaceGone reports whether the workspace is archived or no longer
// retrievable, in which case its memberships cannot be managed any more. The
// API answers member reads and removals for an archived workspace with 400,
// so callers use this after any non-404 error.
func (e *external) workspaceGone(ctx context.Context, workspaceID string) bool {
	ws, err := e.client.Beta.Organization.Workspaces.Get(ctx, workspaceID)
	if err != nil {
		var apiErr *anthropic.Error
		return errors.As(err, &apiErr) && apiErr.StatusCode == 404
	}
	return !ws.ArchivedAt.IsZero()
}

func workspaceIDOf(m *v1beta1.WorkspaceMember) string {
	if m.Spec.ForProvider.WorkspaceID == nil {
		return ""
	}
	return *m.Spec.ForProvider.WorkspaceID
}

// isUpToDate performs a structured diff between spec.forProvider and
// status.atProvider, skipping nil ForProvider fields and ForProvider-only
// fields that have no AtProvider counterpart (the Ref/Selector fields).
func isUpToDate(m *v1beta1.WorkspaceMember) bool {
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
	return clients.IsSubsetEqual(fp, ap)
}

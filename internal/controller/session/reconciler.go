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

// Package session implements the Crossplane managed reconciler for the
// Anthropic Sessions beta API.
package session

import (
	"context"
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

	betav1alpha1 "github.com/jonasz-lasut/provider-anthropic-platform/apis/beta/v1alpha1"
	"github.com/jonasz-lasut/provider-anthropic-platform/internal/clients"
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
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(betav1alpha1.SessionKind)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&betav1alpha1.Session{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(betav1alpha1.SessionGroupVersionKind),
			managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
			managed.WithLogger(o.Logger.WithValues("controller", name)),
			managed.WithPollInterval(o.PollInterval),
			managed.WithManagementPolicies(),
		))
}

// SetupGated registers the Session controller to start only once the
// Session CRD is established.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(err)
		}
	}, betav1alpha1.SessionGroupVersionKind)
	return nil
}

// connector builds an ExternalClient for each reconcile.
type connector struct {
	kube client.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	sess, ok := mg.(*betav1alpha1.Session)
	if !ok {
		return nil, xperrors.New(errNotSession)
	}

	cl, err := clients.NewClient(ctx, c.kube, sess)
	if err != nil {
		return nil, xperrors.Wrap(err, errNewClient)
	}

	return &external{client: cl}, nil
}

// external implements managed.ExternalClient for Anthropic Sessions.
type external struct {
	client *anthropic.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	sess, ok := mg.(*betav1alpha1.Session)
	if !ok {
		return managed.ExternalObservation{}, xperrors.New(errNotSession)
	}

	// The external-name annotation holds the Anthropic session ID once created.
	// If it still equals the k8s object name the resource has never been created.
	sessID := meta.GetExternalName(sess)
	if sessID == sess.GetName() {
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

	// Populate observed state.
	sess.Status.AtProvider.ID = &resp.ID
	createdAt := resp.CreatedAt.String()
	updatedAt := resp.UpdatedAt.String()
	sess.Status.AtProvider.CreatedAt = &createdAt
	sess.Status.AtProvider.UpdatedAt = &updatedAt
	if !resp.ArchivedAt.IsZero() {
		archivedAt := resp.ArchivedAt.String()
		sess.Status.AtProvider.ArchivedAt = &archivedAt
	}
	sess.Status.AtProvider.EnvironmentID = &resp.EnvironmentID
	sess.Status.AtProvider.AgentID = &resp.Agent.ID
	status := string(resp.Status)
	sess.Status.AtProvider.Status = &status

	sess.SetConditions(xpv1.Available())

	upToDate := isUpToDate(sess, resp)
	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate,
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	sess, ok := mg.(*betav1alpha1.Session)
	if !ok {
		return managed.ExternalCreation{}, xperrors.New(errNotSession)
	}

	params := buildNewParams(sess.Spec.ForProvider)
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
	sess, ok := mg.(*betav1alpha1.Session)
	if !ok {
		return managed.ExternalUpdate{}, xperrors.New(errNotSession)
	}

	sessID := meta.GetExternalName(sess)
	if sessID == sess.GetName() {
		return managed.ExternalUpdate{}, xperrors.New("external name not yet set; skipping update")
	}

	params := buildUpdateParams(sess.Spec.ForProvider)
	if _, err := e.client.Beta.Sessions.Update(ctx, sessID, params); err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	sess, ok := mg.(*betav1alpha1.Session)
	if !ok {
		return managed.ExternalDelete{}, xperrors.New(errNotSession)
	}

	sessID := meta.GetExternalName(sess)
	if sessID == sess.GetName() {
		return managed.ExternalDelete{}, nil
	}

	policy := betav1alpha1.DeletionPolicyArchive
	if sess.Spec.ForProvider.AnthropicDeletionPolicy != nil {
		policy = *sess.Spec.ForProvider.AnthropicDeletionPolicy
	}

	var err error
	if policy == betav1alpha1.DeletionPolicyDelete {
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

// buildNewParams converts ForProvider into the SDK create params.
func buildNewParams(p betav1alpha1.SessionParameters) anthropic.BetaSessionNewParams {
	params := anthropic.BetaSessionNewParams{}

	// Agent: use versioned form when a version is pinned, string form otherwise.
	if p.AgentVersion != nil && p.AgentID != nil {
		params.Agent = anthropic.BetaSessionNewParamsAgentUnion{
			OfBetaManagedAgentsAgents: &anthropic.BetaManagedAgentsAgentParams{
				ID:      *p.AgentID,
				Type:    anthropic.BetaManagedAgentsAgentParamsTypeAgent,
				Version: anthropic.Int(*p.AgentVersion),
			},
		}
	} else if p.AgentID != nil {
		params.Agent = anthropic.BetaSessionNewParamsAgentUnion{
			OfString: anthropic.String(*p.AgentID),
		}
	}

	if p.EnvironmentID != nil {
		params.EnvironmentID = *p.EnvironmentID
	}
	if p.Title != nil {
		params.Title = anthropic.String(*p.Title)
	}
	if p.Metadata != nil {
		params.Metadata = p.Metadata
	}
	if p.VaultIDs != nil {
		params.VaultIDs = p.VaultIDs
	}

	for _, res := range p.Resources {
		union := buildResourceParam(res)
		params.Resources = append(params.Resources, union)
	}

	return params
}

// buildUpdateParams converts the mutable subset of ForProvider into SDK update params.
// Only Title and Metadata are updatable; all other fields are immutable after creation.
func buildUpdateParams(p betav1alpha1.SessionParameters) anthropic.BetaSessionUpdateParams {
	params := anthropic.BetaSessionUpdateParams{}
	if p.Title != nil {
		params.Title = anthropic.String(*p.Title)
	}
	if p.Metadata != nil {
		params.Metadata = p.Metadata
	}
	return params
}

// buildResourceParam converts a SessionResource into the SDK union type.
func buildResourceParam(res betav1alpha1.SessionResource) anthropic.BetaSessionNewParamsResourceUnion {
	resType := ""
	if res.Type != nil {
		resType = *res.Type
	}
	switch resType {
	case "github_repository":
		ghParams := anthropic.BetaManagedAgentsGitHubRepositoryResourceParams{
			Type: anthropic.BetaManagedAgentsGitHubRepositoryResourceParamsTypeGitHubRepository,
		}
		if res.URL != nil {
			ghParams.URL = *res.URL
		}
		if res.AuthorizationToken != nil {
			ghParams.AuthorizationToken = *res.AuthorizationToken
		}
		if res.MountPath != nil {
			ghParams.MountPath = anthropic.String(*res.MountPath)
		}
		if res.Checkout != nil {
			checkoutType := ""
			if res.Checkout.Type != nil {
				checkoutType = *res.Checkout.Type
			}
			switch checkoutType {
			case "branch":
				if res.Checkout.Name != nil {
					ghParams.Checkout = anthropic.BetaManagedAgentsGitHubRepositoryResourceParamsCheckoutUnion{
						OfBranch: &anthropic.BetaManagedAgentsBranchCheckoutParam{
							Name: *res.Checkout.Name,
							Type: anthropic.BetaManagedAgentsBranchCheckoutTypeBranch,
						},
					}
				}
			case "commit":
				if res.Checkout.Sha != nil {
					ghParams.Checkout = anthropic.BetaManagedAgentsGitHubRepositoryResourceParamsCheckoutUnion{
						OfCommit: &anthropic.BetaManagedAgentsCommitCheckoutParam{
							Sha:  *res.Checkout.Sha,
							Type: anthropic.BetaManagedAgentsCommitCheckoutTypeCommit,
						},
					}
				}
			}
		}
		return anthropic.BetaSessionNewParamsResourceUnion{OfGitHubRepository: &ghParams}

	case "file":
		fileParams := anthropic.BetaManagedAgentsFileResourceParams{
			Type: anthropic.BetaManagedAgentsFileResourceParamsTypeFile,
		}
		if res.FileID != nil {
			fileParams.FileID = *res.FileID
		}
		if res.MountPath != nil {
			fileParams.MountPath = anthropic.String(*res.MountPath)
		}
		return anthropic.BetaSessionNewParamsResourceUnion{OfFile: &fileParams}

	case "memory_store":
		msParams := anthropic.BetaManagedAgentsMemoryStoreResourceParam{
			Type: anthropic.BetaManagedAgentsMemoryStoreResourceParamTypeMemoryStore,
		}
		if res.MemoryStoreID != nil {
			msParams.MemoryStoreID = *res.MemoryStoreID
		}
		if res.Instructions != nil {
			msParams.Instructions = anthropic.String(*res.Instructions)
		}
		if res.Access != nil {
			msParams.Access = anthropic.BetaManagedAgentsMemoryStoreResourceParamAccess(*res.Access)
		}
		return anthropic.BetaSessionNewParamsResourceUnion{OfMemoryStore: &msParams}
	}

	return anthropic.BetaSessionNewParamsResourceUnion{}
}

// isUpToDate compares the mutable desired state against the observed session.
// Resources, VaultIDs, EnvironmentID, and AgentID are immutable after creation
// and are not compared here.
func isUpToDate(sess *betav1alpha1.Session, resp *anthropic.BetaManagedAgentsSession) bool {
	p := sess.Spec.ForProvider

	if p.Title != nil && *p.Title != resp.Title {
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

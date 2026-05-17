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

// Package vaultcredential implements the Crossplane managed reconciler for
// the Anthropic VaultCredential beta API.
package vaultcredential

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
	errNotVaultCredential = "managed resource is not a VaultCredential"
	errNewClient          = "cannot build Anthropic client"
	errObserve            = "cannot observe VaultCredential"
	errCreate             = "cannot create VaultCredential"
	errUpdate             = "cannot update VaultCredential"
	errDelete             = "cannot delete/archive VaultCredential"
	errMissingVault       = "spec.forProvider.vaultId not resolved"
)

// Setup adds a controller for VaultCredential to the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(betav1alpha1.VaultCredentialKind)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&betav1alpha1.VaultCredential{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(betav1alpha1.VaultCredentialGroupVersionKind),
			managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
			managed.WithLogger(o.Logger.WithValues("controller", name)),
			managed.WithPollInterval(o.PollInterval),
			managed.WithManagementPolicies(),
		))
}

// SetupGated registers the VaultCredential controller to start only once the
// VaultCredential CRD is established.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(err)
		}
	}, betav1alpha1.VaultCredentialGroupVersionKind)
	return nil
}

// connector builds an ExternalClient for each reconcile.
type connector struct {
	kube client.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	vc, ok := mg.(*betav1alpha1.VaultCredential)
	if !ok {
		return nil, xperrors.New(errNotVaultCredential)
	}

	cl, err := clients.NewClient(ctx, c.kube, vc)
	if err != nil {
		return nil, xperrors.Wrap(err, errNewClient)
	}

	return &external{client: cl, kube: c.kube}, nil
}

// external implements managed.ExternalClient for Anthropic VaultCredentials.
type external struct {
	client *anthropic.Client
	kube   client.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	vc, ok := mg.(*betav1alpha1.VaultCredential)
	if !ok {
		return managed.ExternalObservation{}, xperrors.New(errNotVaultCredential)
	}

	credID := meta.GetExternalName(vc)
	if credID == vc.GetName() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	if vc.Spec.ForProvider.VaultID == nil || *vc.Spec.ForProvider.VaultID == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	resp, err := e.client.Beta.Vaults.Credentials.Get(ctx, credID, anthropic.BetaVaultCredentialGetParams{
		VaultID: *vc.Spec.ForProvider.VaultID,
	})
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, xperrors.Wrap(err, errObserve)
	}

	// Archived credentials are treated as deleted — Crossplane will re-create them.
	if !resp.ArchivedAt.IsZero() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	vc.Status.AtProvider.ID = &resp.ID
	vc.Status.AtProvider.VaultID = &resp.VaultID
	createdAt := resp.CreatedAt.Format(time.RFC3339)
	updatedAt := resp.UpdatedAt.Format(time.RFC3339)
	vc.Status.AtProvider.CreatedAt = &createdAt
	vc.Status.AtProvider.UpdatedAt = &updatedAt
	if !resp.ArchivedAt.IsZero() {
		archivedAt := resp.ArchivedAt.Format(time.RFC3339)
		vc.Status.AtProvider.ArchivedAt = &archivedAt
	}

	vc.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isUpToDate(vc, resp),
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	vc, ok := mg.(*betav1alpha1.VaultCredential)
	if !ok {
		return managed.ExternalCreation{}, xperrors.New(errNotVaultCredential)
	}

	if vc.Spec.ForProvider.VaultID == nil || *vc.Spec.ForProvider.VaultID == "" {
		return managed.ExternalCreation{}, xperrors.New(errMissingVault)
	}

	params, err := buildNewParams(ctx, e.kube, vc.Spec.ForProvider, vc.GetNamespace())
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}
	resp, err := e.client.Beta.Vaults.Credentials.New(ctx, *vc.Spec.ForProvider.VaultID, params)
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}

	meta.SetExternalName(vc, resp.ID)
	vc.Status.AtProvider.ID = &resp.ID
	vc.Status.AtProvider.VaultID = &resp.VaultID

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	vc, ok := mg.(*betav1alpha1.VaultCredential)
	if !ok {
		return managed.ExternalUpdate{}, xperrors.New(errNotVaultCredential)
	}

	credID := meta.GetExternalName(vc)
	if credID == vc.GetName() {
		return managed.ExternalUpdate{}, xperrors.New("external name not yet set; skipping update")
	}

	if vc.Spec.ForProvider.VaultID == nil || *vc.Spec.ForProvider.VaultID == "" {
		return managed.ExternalUpdate{}, xperrors.New(errMissingVault)
	}

	params, err := buildUpdateParams(ctx, e.kube, vc.Spec.ForProvider, vc.GetNamespace())
	if err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}
	if _, err := e.client.Beta.Vaults.Credentials.Update(ctx, credID, params); err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	vc, ok := mg.(*betav1alpha1.VaultCredential)
	if !ok {
		return managed.ExternalDelete{}, xperrors.New(errNotVaultCredential)
	}

	credID := meta.GetExternalName(vc)
	if credID == vc.GetName() {
		return managed.ExternalDelete{}, nil
	}

	if vc.Spec.ForProvider.VaultID == nil || *vc.Spec.ForProvider.VaultID == "" {
		// Without a resolved vault ID we cannot target the API; treat as no-op.
		return managed.ExternalDelete{}, nil
	}

	policy := betav1alpha1.DeletionPolicyArchive
	if vc.Spec.ForProvider.AnthropicDeletionPolicy != nil {
		policy = *vc.Spec.ForProvider.AnthropicDeletionPolicy
	}

	var err error
	if policy == betav1alpha1.DeletionPolicyDelete {
		_, err = e.client.Beta.Vaults.Credentials.Delete(ctx, credID, anthropic.BetaVaultCredentialDeleteParams{
			VaultID: *vc.Spec.ForProvider.VaultID,
		})
	} else {
		_, err = e.client.Beta.Vaults.Credentials.Archive(ctx, credID, anthropic.BetaVaultCredentialArchiveParams{
			VaultID: *vc.Spec.ForProvider.VaultID,
		})
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
func buildNewParams(ctx context.Context, kube client.Client, p betav1alpha1.VaultCredentialParameters, namespace string) (anthropic.BetaVaultCredentialNewParams, error) {
	auth, err := buildNewAuthUnion(ctx, kube, p.Auth, namespace)
	if err != nil {
		return anthropic.BetaVaultCredentialNewParams{}, err
	}
	params := anthropic.BetaVaultCredentialNewParams{
		Auth: auth,
	}
	if p.DisplayName != nil {
		params.DisplayName = anthropic.String(*p.DisplayName)
	}
	if p.Metadata != nil {
		params.Metadata = p.Metadata
	}
	return params, nil
}

// buildUpdateParams converts ForProvider into the SDK update params.
func buildUpdateParams(ctx context.Context, kube client.Client, p betav1alpha1.VaultCredentialParameters, namespace string) (anthropic.BetaVaultCredentialUpdateParams, error) {
	auth, err := buildUpdateAuthUnion(ctx, kube, p.Auth, namespace)
	if err != nil {
		return anthropic.BetaVaultCredentialUpdateParams{}, err
	}
	params := anthropic.BetaVaultCredentialUpdateParams{
		Auth: auth,
	}
	if p.VaultID != nil {
		params.VaultID = *p.VaultID
	}
	if p.DisplayName != nil {
		params.DisplayName = anthropic.String(*p.DisplayName)
	}
	if p.Metadata != nil {
		params.Metadata = p.Metadata
	}
	return params, nil
}

// buildNewAuthUnion converts the auth spec into the SDK create union variant.
func buildNewAuthUnion(ctx context.Context, kube client.Client, a betav1alpha1.VaultCredentialAuth, namespace string) (anthropic.BetaVaultCredentialNewParamsAuthUnion, error) {
	authType := ""
	if a.Type != nil {
		authType = *a.Type
	}
	mcpServerURL := ""
	if a.MCPServerURL != nil {
		mcpServerURL = *a.MCPServerURL
	}
	switch authType {
	case "static_bearer":
		sb := &anthropic.BetaManagedAgentsStaticBearerCreateParams{
			MCPServerURL: mcpServerURL,
			Type:         anthropic.BetaManagedAgentsStaticBearerCreateParamsTypeStaticBearer,
		}
		token, err := clients.ResolveLocalSecretKey(ctx, kube, a.TokenSecretRef, namespace)
		if err != nil {
			return anthropic.BetaVaultCredentialNewParamsAuthUnion{}, err
		}
		if token != "" {
			sb.Token = token
		}
		return anthropic.BetaVaultCredentialNewParamsAuthUnion{OfStaticBearer: sb}, nil

	case "mcp_oauth":
		oauth := &anthropic.BetaManagedAgentsMCPOAuthCreateParams{
			MCPServerURL: mcpServerURL,
			Type:         anthropic.BetaManagedAgentsMCPOAuthCreateParamsTypeMCPOAuth,
		}
		accessToken, err := clients.ResolveLocalSecretKey(ctx, kube, a.AccessTokenSecretRef, namespace)
		if err != nil {
			return anthropic.BetaVaultCredentialNewParamsAuthUnion{}, err
		}
		if accessToken != "" {
			oauth.AccessToken = accessToken
		}
		if a.ExpiresAt != nil {
			if t, err := time.Parse(time.RFC3339, *a.ExpiresAt); err == nil {
				oauth.ExpiresAt = anthropic.Time(t)
			}
		}
		if a.Refresh != nil {
			refresh, err := buildRefreshCreateParams(ctx, kube, *a.Refresh, namespace)
			if err != nil {
				return anthropic.BetaVaultCredentialNewParamsAuthUnion{}, err
			}
			oauth.Refresh = refresh
		}
		return anthropic.BetaVaultCredentialNewParamsAuthUnion{OfMCPOAuth: oauth}, nil
	}

	return anthropic.BetaVaultCredentialNewParamsAuthUnion{}, nil
}

// buildUpdateAuthUnion converts the auth spec into the SDK update union variant.
func buildUpdateAuthUnion(ctx context.Context, kube client.Client, a betav1alpha1.VaultCredentialAuth, namespace string) (anthropic.BetaVaultCredentialUpdateParamsAuthUnion, error) {
	authType := ""
	if a.Type != nil {
		authType = *a.Type
	}
	switch authType {
	case "static_bearer":
		sb := &anthropic.BetaManagedAgentsStaticBearerUpdateParams{
			Type: anthropic.BetaManagedAgentsStaticBearerUpdateParamsTypeStaticBearer,
		}
		token, err := clients.ResolveLocalSecretKey(ctx, kube, a.TokenSecretRef, namespace)
		if err != nil {
			return anthropic.BetaVaultCredentialUpdateParamsAuthUnion{}, err
		}
		if token != "" {
			sb.Token = anthropic.String(token)
		}
		return anthropic.BetaVaultCredentialUpdateParamsAuthUnion{OfStaticBearer: sb}, nil

	case "mcp_oauth":
		oauth := &anthropic.BetaManagedAgentsMCPOAuthUpdateParams{
			Type: anthropic.BetaManagedAgentsMCPOAuthUpdateParamsTypeMCPOAuth,
		}
		accessToken, err := clients.ResolveLocalSecretKey(ctx, kube, a.AccessTokenSecretRef, namespace)
		if err != nil {
			return anthropic.BetaVaultCredentialUpdateParamsAuthUnion{}, err
		}
		if accessToken != "" {
			oauth.AccessToken = anthropic.String(accessToken)
		}
		if a.ExpiresAt != nil {
			if t, err := time.Parse(time.RFC3339, *a.ExpiresAt); err == nil {
				oauth.ExpiresAt = anthropic.Time(t)
			}
		}
		if a.Refresh != nil {
			refresh, err := buildRefreshUpdateParams(ctx, kube, *a.Refresh, namespace)
			if err != nil {
				return anthropic.BetaVaultCredentialUpdateParamsAuthUnion{}, err
			}
			oauth.Refresh = refresh
		}
		return anthropic.BetaVaultCredentialUpdateParamsAuthUnion{OfMCPOAuth: oauth}, nil
	}

	return anthropic.BetaVaultCredentialUpdateParamsAuthUnion{}, nil
}

// buildRefreshCreateParams converts the spec refresh block into SDK create params.
func buildRefreshCreateParams(ctx context.Context, kube client.Client, r betav1alpha1.VaultCredentialRefresh, namespace string) (anthropic.BetaManagedAgentsMCPOAuthRefreshParams, error) {
	auth, err := buildTokenEndpointAuthCreate(ctx, kube, r.TokenEndpointAuth, namespace)
	if err != nil {
		return anthropic.BetaManagedAgentsMCPOAuthRefreshParams{}, err
	}
	out := anthropic.BetaManagedAgentsMCPOAuthRefreshParams{
		TokenEndpointAuth: auth,
	}
	if r.ClientID != nil {
		out.ClientID = *r.ClientID
	}
	refresh, err := clients.ResolveLocalSecretKey(ctx, kube, r.RefreshTokenSecretRef, namespace)
	if err != nil {
		return anthropic.BetaManagedAgentsMCPOAuthRefreshParams{}, err
	}
	if refresh != "" {
		out.RefreshToken = refresh
	}
	if r.TokenEndpoint != nil {
		out.TokenEndpoint = *r.TokenEndpoint
	}
	if r.Resource != nil {
		out.Resource = anthropic.String(*r.Resource)
	}
	if r.Scope != nil {
		out.Scope = anthropic.String(*r.Scope)
	}
	return out, nil
}

// buildRefreshUpdateParams converts the spec refresh block into SDK update params.
// The update API only supports rotating refresh_token / scope / token_endpoint_auth;
// client_id and token_endpoint are immutable.
func buildRefreshUpdateParams(ctx context.Context, kube client.Client, r betav1alpha1.VaultCredentialRefresh, namespace string) (anthropic.BetaManagedAgentsMCPOAuthRefreshUpdateParams, error) {
	auth, err := buildTokenEndpointAuthUpdate(ctx, kube, r.TokenEndpointAuth, namespace)
	if err != nil {
		return anthropic.BetaManagedAgentsMCPOAuthRefreshUpdateParams{}, err
	}
	out := anthropic.BetaManagedAgentsMCPOAuthRefreshUpdateParams{
		TokenEndpointAuth: auth,
	}
	refresh, err := clients.ResolveLocalSecretKey(ctx, kube, r.RefreshTokenSecretRef, namespace)
	if err != nil {
		return anthropic.BetaManagedAgentsMCPOAuthRefreshUpdateParams{}, err
	}
	if refresh != "" {
		out.RefreshToken = anthropic.String(refresh)
	}
	if r.Scope != nil {
		out.Scope = anthropic.String(*r.Scope)
	}
	return out, nil
}

// buildTokenEndpointAuthCreate selects the token-endpoint-auth union variant
// for create requests.
func buildTokenEndpointAuthCreate(ctx context.Context, kube client.Client, t betav1alpha1.VaultCredentialTokenEndpointAuth, namespace string) (anthropic.BetaManagedAgentsMCPOAuthRefreshParamsTokenEndpointAuthUnion, error) {
	tType := ""
	if t.Type != nil {
		tType = *t.Type
	}
	if tType == "none" {
		return anthropic.BetaManagedAgentsMCPOAuthRefreshParamsTokenEndpointAuthUnion{
			OfNone: &anthropic.BetaManagedAgentsTokenEndpointAuthNoneParam{
				Type: anthropic.BetaManagedAgentsTokenEndpointAuthNoneParamTypeNone,
			},
		}, nil
	}
	clientSecret, err := clients.ResolveLocalSecretKey(ctx, kube, t.ClientSecretSecretRef, namespace)
	if err != nil {
		return anthropic.BetaManagedAgentsMCPOAuthRefreshParamsTokenEndpointAuthUnion{}, err
	}
	switch tType {
	case "client_secret_basic":
		p := &anthropic.BetaManagedAgentsTokenEndpointAuthBasicParam{
			Type: anthropic.BetaManagedAgentsTokenEndpointAuthBasicParamTypeClientSecretBasic,
		}
		if clientSecret != "" {
			p.ClientSecret = clientSecret
		}
		return anthropic.BetaManagedAgentsMCPOAuthRefreshParamsTokenEndpointAuthUnion{OfClientSecretBasic: p}, nil
	case "client_secret_post":
		p := &anthropic.BetaManagedAgentsTokenEndpointAuthPostParam{
			Type: anthropic.BetaManagedAgentsTokenEndpointAuthPostParamTypeClientSecretPost,
		}
		if clientSecret != "" {
			p.ClientSecret = clientSecret
		}
		return anthropic.BetaManagedAgentsMCPOAuthRefreshParamsTokenEndpointAuthUnion{OfClientSecretPost: p}, nil
	}
	return anthropic.BetaManagedAgentsMCPOAuthRefreshParamsTokenEndpointAuthUnion{}, nil
}

// buildTokenEndpointAuthUpdate selects the token-endpoint-auth union variant
// for update requests. The update API does not support the "none" variant.
func buildTokenEndpointAuthUpdate(ctx context.Context, kube client.Client, t betav1alpha1.VaultCredentialTokenEndpointAuth, namespace string) (anthropic.BetaManagedAgentsMCPOAuthRefreshUpdateParamsTokenEndpointAuthUnion, error) {
	tType := ""
	if t.Type != nil {
		tType = *t.Type
	}
	if tType != "client_secret_basic" && tType != "client_secret_post" {
		return anthropic.BetaManagedAgentsMCPOAuthRefreshUpdateParamsTokenEndpointAuthUnion{}, nil
	}
	clientSecret, err := clients.ResolveLocalSecretKey(ctx, kube, t.ClientSecretSecretRef, namespace)
	if err != nil {
		return anthropic.BetaManagedAgentsMCPOAuthRefreshUpdateParamsTokenEndpointAuthUnion{}, err
	}
	switch tType {
	case "client_secret_basic":
		p := &anthropic.BetaManagedAgentsTokenEndpointAuthBasicUpdateParam{
			Type: anthropic.BetaManagedAgentsTokenEndpointAuthBasicUpdateParamTypeClientSecretBasic,
		}
		if clientSecret != "" {
			p.ClientSecret = anthropic.String(clientSecret)
		}
		return anthropic.BetaManagedAgentsMCPOAuthRefreshUpdateParamsTokenEndpointAuthUnion{OfClientSecretBasic: p}, nil
	case "client_secret_post":
		p := &anthropic.BetaManagedAgentsTokenEndpointAuthPostUpdateParam{
			Type: anthropic.BetaManagedAgentsTokenEndpointAuthPostUpdateParamTypeClientSecretPost,
		}
		if clientSecret != "" {
			p.ClientSecret = anthropic.String(clientSecret)
		}
		return anthropic.BetaManagedAgentsMCPOAuthRefreshUpdateParamsTokenEndpointAuthUnion{OfClientSecretPost: p}, nil
	}
	return anthropic.BetaManagedAgentsMCPOAuthRefreshUpdateParamsTokenEndpointAuthUnion{}, nil
}

// isUpToDate compares the desired state with the observed credential. The
// auth payload itself (token, access token) is write-only and never returned,
// so we cannot diff it — callers who want to rotate must touch the spec to
// trigger an Update.
func isUpToDate(vc *betav1alpha1.VaultCredential, resp *anthropic.BetaManagedAgentsCredential) bool {
	p := vc.Spec.ForProvider

	if p.DisplayName != nil && *p.DisplayName != resp.DisplayName {
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

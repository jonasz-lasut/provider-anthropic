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

package v1alpha1

import (
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
)

// VaultCredentialConversionContext carries pre-resolved secret values for
// VaultCredential SDK param construction.
type VaultCredentialConversionContext struct {
	BearerToken  string // static_bearer: resolved from TokenSecretRef
	AccessToken  string // mcp_oauth: resolved from AccessTokenSecretRef
	RefreshToken string // mcp_oauth refresh: resolved from RefreshTokenSecretRef
	ClientSecret string // mcp_oauth refresh: resolved from ClientSecretSecretRef
}

func (r *VaultCredential) ToAnthropicNew(ctx *VaultCredentialConversionContext) (anthropic.BetaVaultCredentialNewParams, error) {
	p := r.Spec.ForProvider
	auth, err := vcNewAuthUnion(p.Auth, ctx)
	if err != nil {
		return anthropic.BetaVaultCredentialNewParams{}, err
	}
	params := anthropic.BetaVaultCredentialNewParams{Auth: auth}
	if p.DisplayName != nil {
		params.DisplayName = anthropic.String(*p.DisplayName)
	}
	if p.Metadata != nil {
		params.Metadata = p.Metadata
	}
	return params, nil
}

func (r *VaultCredential) ToAnthropicUpdate(ctx *VaultCredentialConversionContext) (anthropic.BetaVaultCredentialUpdateParams, error) {
	p := r.Spec.ForProvider
	auth, err := vcUpdateAuthUnion(p.Auth, ctx)
	if err != nil {
		return anthropic.BetaVaultCredentialUpdateParams{}, err
	}
	params := anthropic.BetaVaultCredentialUpdateParams{Auth: auth}
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

func (r *VaultCredential) FromAnthropicObservation(resp anthropic.BetaManagedAgentsCredential) {
	r.Status.AtProvider.ID = &resp.ID
	r.Status.AtProvider.VaultID = &resp.VaultID
	r.Status.AtProvider.DisplayName = &resp.DisplayName
	r.Status.AtProvider.Metadata = resp.Metadata
	authType := resp.Auth.Type
	authURL := resp.Auth.MCPServerURL
	r.Status.AtProvider.Auth = &VaultCredentialAuthObservation{
		Type:         &authType,
		MCPServerURL: &authURL,
	}
	createdAt := resp.CreatedAt.Format(time.RFC3339)
	r.Status.AtProvider.CreatedAt = &createdAt
	updatedAt := resp.UpdatedAt.Format(time.RFC3339)
	r.Status.AtProvider.UpdatedAt = &updatedAt
	// ArchivedAt intentionally omitted
}

func vcNewAuthUnion(a VaultCredentialAuth, ctx *VaultCredentialConversionContext) (anthropic.BetaVaultCredentialNewParamsAuthUnion, error) {
	authType := ""
	if a.Type != nil {
		authType = *a.Type
	}
	mcpServerURL := ""
	if a.MCPServerURL != nil {
		mcpServerURL = *a.MCPServerURL
	}
	token, accessToken := "", ""
	if ctx != nil {
		token = ctx.BearerToken
		accessToken = ctx.AccessToken
	}

	switch authType {
	case "static_bearer":
		sb := &anthropic.BetaManagedAgentsStaticBearerCreateParams{
			MCPServerURL: mcpServerURL,
			Type:         anthropic.BetaManagedAgentsStaticBearerCreateParamsTypeStaticBearer,
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
		if accessToken != "" {
			oauth.AccessToken = accessToken
		}
		if a.ExpiresAt != nil {
			if t, err := time.Parse(time.RFC3339, *a.ExpiresAt); err == nil {
				oauth.ExpiresAt = anthropic.Time(t)
			}
		}
		if a.Refresh != nil && ctx != nil {
			oauth.Refresh = vcNewRefreshParams(*a.Refresh, ctx)
		}
		return anthropic.BetaVaultCredentialNewParamsAuthUnion{OfMCPOAuth: oauth}, nil
	}
	return anthropic.BetaVaultCredentialNewParamsAuthUnion{}, nil
}

func vcNewRefreshParams(r VaultCredentialRefresh, ctx *VaultCredentialConversionContext) anthropic.BetaManagedAgentsMCPOAuthRefreshParams {
	params := anthropic.BetaManagedAgentsMCPOAuthRefreshParams{
		TokenEndpointAuth: vcNewTEPUnion(r.TokenEndpointAuth, ctx.ClientSecret),
	}
	if r.ClientID != nil {
		params.ClientID = *r.ClientID
	}
	if ctx.RefreshToken != "" {
		params.RefreshToken = ctx.RefreshToken
	}
	if r.TokenEndpoint != nil {
		params.TokenEndpoint = *r.TokenEndpoint
	}
	if r.Resource != nil {
		params.Resource = anthropic.String(*r.Resource)
	}
	if r.Scope != nil {
		params.Scope = anthropic.String(*r.Scope)
	}
	return params
}

func vcNewTEPUnion(a VaultCredentialTokenEndpointAuth, clientSecret string) anthropic.BetaManagedAgentsMCPOAuthRefreshParamsTokenEndpointAuthUnion {
	tepType := ""
	if a.Type != nil {
		tepType = *a.Type
	}
	switch tepType {
	case "none":
		return anthropic.BetaManagedAgentsMCPOAuthRefreshParamsTokenEndpointAuthUnion{
			OfNone: &anthropic.BetaManagedAgentsTokenEndpointAuthNoneParam{
				Type: anthropic.BetaManagedAgentsTokenEndpointAuthNoneParamTypeNone,
			},
		}
	case "client_secret_basic":
		p := &anthropic.BetaManagedAgentsTokenEndpointAuthBasicParam{
			Type: anthropic.BetaManagedAgentsTokenEndpointAuthBasicParamTypeClientSecretBasic,
		}
		if clientSecret != "" {
			p.ClientSecret = clientSecret
		}
		return anthropic.BetaManagedAgentsMCPOAuthRefreshParamsTokenEndpointAuthUnion{OfClientSecretBasic: p}
	case "client_secret_post":
		p := &anthropic.BetaManagedAgentsTokenEndpointAuthPostParam{
			Type: anthropic.BetaManagedAgentsTokenEndpointAuthPostParamTypeClientSecretPost,
		}
		if clientSecret != "" {
			p.ClientSecret = clientSecret
		}
		return anthropic.BetaManagedAgentsMCPOAuthRefreshParamsTokenEndpointAuthUnion{OfClientSecretPost: p}
	}
	return anthropic.BetaManagedAgentsMCPOAuthRefreshParamsTokenEndpointAuthUnion{}
}

func vcUpdateAuthUnion(a VaultCredentialAuth, ctx *VaultCredentialConversionContext) (anthropic.BetaVaultCredentialUpdateParamsAuthUnion, error) {
	authType := ""
	if a.Type != nil {
		authType = *a.Type
	}
	token, accessToken := "", ""
	if ctx != nil {
		token = ctx.BearerToken
		accessToken = ctx.AccessToken
	}

	switch authType {
	case "static_bearer":
		sb := &anthropic.BetaManagedAgentsStaticBearerUpdateParams{
			Type: anthropic.BetaManagedAgentsStaticBearerUpdateParamsTypeStaticBearer,
		}
		if token != "" {
			sb.Token = anthropic.String(token)
		}
		return anthropic.BetaVaultCredentialUpdateParamsAuthUnion{OfStaticBearer: sb}, nil

	case "mcp_oauth":
		oauth := &anthropic.BetaManagedAgentsMCPOAuthUpdateParams{
			Type: anthropic.BetaManagedAgentsMCPOAuthUpdateParamsTypeMCPOAuth,
		}
		if accessToken != "" {
			oauth.AccessToken = anthropic.String(accessToken)
		}
		if a.ExpiresAt != nil {
			if t, err := time.Parse(time.RFC3339, *a.ExpiresAt); err == nil {
				oauth.ExpiresAt = anthropic.Time(t)
			}
		}
		if a.Refresh != nil && ctx != nil {
			oauth.Refresh = vcUpdateRefreshParams(*a.Refresh, ctx)
		}
		return anthropic.BetaVaultCredentialUpdateParamsAuthUnion{OfMCPOAuth: oauth}, nil
	}
	return anthropic.BetaVaultCredentialUpdateParamsAuthUnion{}, nil
}

func vcUpdateRefreshParams(r VaultCredentialRefresh, ctx *VaultCredentialConversionContext) anthropic.BetaManagedAgentsMCPOAuthRefreshUpdateParams {
	params := anthropic.BetaManagedAgentsMCPOAuthRefreshUpdateParams{
		TokenEndpointAuth: vcUpdateTEPUnion(r.TokenEndpointAuth, ctx.ClientSecret),
	}
	if ctx.RefreshToken != "" {
		params.RefreshToken = anthropic.String(ctx.RefreshToken)
	}
	if r.Scope != nil {
		params.Scope = anthropic.String(*r.Scope)
	}
	return params
}

func vcUpdateTEPUnion(a VaultCredentialTokenEndpointAuth, clientSecret string) anthropic.BetaManagedAgentsMCPOAuthRefreshUpdateParamsTokenEndpointAuthUnion {
	tepType := ""
	if a.Type != nil {
		tepType = *a.Type
	}
	switch tepType {
	case "client_secret_basic":
		p := &anthropic.BetaManagedAgentsTokenEndpointAuthBasicUpdateParam{
			Type: anthropic.BetaManagedAgentsTokenEndpointAuthBasicUpdateParamTypeClientSecretBasic,
		}
		if clientSecret != "" {
			p.ClientSecret = anthropic.String(clientSecret)
		}
		return anthropic.BetaManagedAgentsMCPOAuthRefreshUpdateParamsTokenEndpointAuthUnion{OfClientSecretBasic: p}
	case "client_secret_post":
		p := &anthropic.BetaManagedAgentsTokenEndpointAuthPostUpdateParam{
			Type: anthropic.BetaManagedAgentsTokenEndpointAuthPostUpdateParamTypeClientSecretPost,
		}
		if clientSecret != "" {
			p.ClientSecret = anthropic.String(clientSecret)
		}
		return anthropic.BetaManagedAgentsMCPOAuthRefreshUpdateParamsTokenEndpointAuthUnion{OfClientSecretPost: p}
	}
	return anthropic.BetaManagedAgentsMCPOAuthRefreshUpdateParamsTokenEndpointAuthUnion{}
}

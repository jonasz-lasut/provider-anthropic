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

package v1alpha1

import (
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
)

// SessionConversionContext carries reconcile-time values needed for Session
// SDK param construction. Pass nil when no secret resolution is needed.
//
// ResourceTokens is indexed positionally against SessionParameters.Resources:
// ResourceTokens[i] is the resolved authorization token for Resources[i].
// For non-github_repository resources or resources without a SecretRef the
// entry is "".
type SessionConversionContext struct {
	ResourceTokens []string
}

func (r *Session) ToAnthropicNew(ctx *SessionConversionContext) anthropic.BetaSessionNewParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaSessionNewParams{}

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
	params.VaultIDs = p.VaultIDs
	for i, res := range p.Resources {
		token := ""
		if ctx != nil && i < len(ctx.ResourceTokens) {
			token = ctx.ResourceTokens[i]
		}
		params.Resources = append(params.Resources, sessionResourceToParam(res, token))
	}
	return params
}

func (r *Session) ToAnthropicUpdate() anthropic.BetaSessionUpdateParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaSessionUpdateParams{}
	if p.Title != nil {
		params.Title = anthropic.String(*p.Title)
	}
	if p.Metadata != nil {
		params.Metadata = p.Metadata
	}
	return params
}

func (r *Session) FromAnthropicObservation(resp anthropic.BetaManagedAgentsSession) {
	r.Status.AtProvider.ID = &resp.ID
	r.Status.AtProvider.Title = &resp.Title
	r.Status.AtProvider.EnvironmentID = &resp.EnvironmentID
	r.Status.AtProvider.Metadata = resp.Metadata
	r.Status.AtProvider.VaultIDs = resp.VaultIDs
	status := string(resp.Status)
	r.Status.AtProvider.Status = &status
	agentID := resp.Agent.ID
	r.Status.AtProvider.AgentID = &agentID
	createdAt := resp.CreatedAt.Format(time.RFC3339)
	r.Status.AtProvider.CreatedAt = &createdAt
	updatedAt := resp.UpdatedAt.Format(time.RFC3339)
	r.Status.AtProvider.UpdatedAt = &updatedAt
	// ArchivedAt intentionally omitted
}

func sessionResourceToParam(res SessionResource, authToken string) anthropic.BetaSessionNewParamsResourceUnion {
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
		if authToken != "" {
			ghParams.AuthorizationToken = authToken
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

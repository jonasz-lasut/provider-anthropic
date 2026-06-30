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

package v1beta1

import (
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
)

// DeploymentConversionContext carries reconcile-time values needed for
// Deployment SDK param construction. Pass nil when no secret resolution is
// needed.
//
// ResourceTokens is indexed positionally against DeploymentParameters.Resources:
// ResourceTokens[i] is the resolved authorization token for Resources[i]. For
// non-github_repository resources or resources without a SecretRef the entry
// is "".
type DeploymentConversionContext struct {
	ResourceTokens []string
}

func (r *Deployment) ToAnthropicNew(ctx *DeploymentConversionContext) anthropic.BetaDeploymentNewParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaDeploymentNewParams{
		Agent: deploymentAgentNewUnion(p.AgentID, p.AgentVersion),
	}
	if p.Name != nil {
		params.Name = *p.Name
	}
	if p.EnvironmentID != nil {
		params.EnvironmentID = *p.EnvironmentID
	}
	if p.Description != nil {
		params.Description = anthropic.String(*p.Description)
	}
	if p.Metadata != nil {
		params.Metadata = p.Metadata
	}
	params.VaultIDs = p.VaultIDs
	if p.Schedule != nil {
		params.Schedule = deploymentScheduleToParam(*p.Schedule)
	}
	for _, ev := range p.InitialEvents {
		params.InitialEvents = append(params.InitialEvents, deploymentInitialEventToParam(ev))
	}
	for i, res := range p.Resources {
		token := resourceToken(ctx, i)
		params.Resources = append(params.Resources, deploymentResourceToNewParam(res, token))
	}
	return params
}

func (r *Deployment) ToAnthropicUpdate(ctx *DeploymentConversionContext) anthropic.BetaDeploymentUpdateParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaDeploymentUpdateParams{
		Agent: deploymentAgentUpdateUnion(p.AgentID, p.AgentVersion),
	}
	if p.Name != nil {
		params.Name = anthropic.String(*p.Name)
	}
	if p.EnvironmentID != nil {
		params.EnvironmentID = anthropic.String(*p.EnvironmentID)
	}
	if p.Description != nil {
		params.Description = anthropic.String(*p.Description)
	}
	if p.Metadata != nil {
		params.Metadata = p.Metadata
	}
	params.VaultIDs = p.VaultIDs
	if p.Schedule != nil {
		params.Schedule = deploymentScheduleToParam(*p.Schedule)
	}
	for _, ev := range p.InitialEvents {
		params.InitialEvents = append(params.InitialEvents, deploymentInitialEventToParam(ev))
	}
	for i, res := range p.Resources {
		token := resourceToken(ctx, i)
		params.Resources = append(params.Resources, deploymentResourceToUpdateParam(res, token))
	}
	return params
}

func (r *Deployment) FromAnthropicObservation(resp anthropic.BetaManagedAgentsDeployment) {
	r.Status.AtProvider.ID = &resp.ID
	r.Status.AtProvider.Name = &resp.Name
	r.Status.AtProvider.Description = &resp.Description
	r.Status.AtProvider.EnvironmentID = &resp.EnvironmentID
	r.Status.AtProvider.VaultIDs = resp.VaultIDs
	r.Status.AtProvider.Metadata = resp.Metadata
	agentID := resp.Agent.ID
	r.Status.AtProvider.AgentID = &agentID
	agentVersion := resp.Agent.Version
	r.Status.AtProvider.AgentVersion = &agentVersion
	status := string(resp.Status)
	r.Status.AtProvider.Status = &status
	paused := resp.Status == anthropic.BetaManagedAgentsDeploymentStatusPaused
	r.Status.AtProvider.Paused = &paused
	r.Status.AtProvider.Schedule = deploymentScheduleFromObservation(resp.Schedule)
	createdAt := resp.CreatedAt.Format(time.RFC3339)
	r.Status.AtProvider.CreatedAt = &createdAt
	updatedAt := resp.UpdatedAt.Format(time.RFC3339)
	r.Status.AtProvider.UpdatedAt = &updatedAt
	// ArchivedAt intentionally omitted; initialEvents and resources are not
	// mirrored (see DeploymentObservation doc).
}

func resourceToken(ctx *DeploymentConversionContext, i int) string {
	if ctx != nil && i < len(ctx.ResourceTokens) {
		return ctx.ResourceTokens[i]
	}
	return ""
}

func deploymentAgentNewUnion(agentID *string, agentVersion *int64) anthropic.BetaDeploymentNewParamsAgentUnion {
	if agentID == nil {
		return anthropic.BetaDeploymentNewParamsAgentUnion{}
	}
	if agentVersion != nil {
		return anthropic.BetaDeploymentNewParamsAgentUnion{
			OfBetaManagedAgentsAgents: &anthropic.BetaManagedAgentsAgentParams{
				ID:      *agentID,
				Type:    anthropic.BetaManagedAgentsAgentParamsTypeAgent,
				Version: anthropic.Int(*agentVersion),
			},
		}
	}
	return anthropic.BetaDeploymentNewParamsAgentUnion{OfString: anthropic.String(*agentID)}
}

func deploymentAgentUpdateUnion(agentID *string, agentVersion *int64) anthropic.BetaDeploymentUpdateParamsAgentUnion {
	if agentID == nil {
		return anthropic.BetaDeploymentUpdateParamsAgentUnion{}
	}
	if agentVersion != nil {
		return anthropic.BetaDeploymentUpdateParamsAgentUnion{
			OfBetaManagedAgentsAgents: &anthropic.BetaManagedAgentsAgentParams{
				ID:      *agentID,
				Type:    anthropic.BetaManagedAgentsAgentParamsTypeAgent,
				Version: anthropic.Int(*agentVersion),
			},
		}
	}
	return anthropic.BetaDeploymentUpdateParamsAgentUnion{OfString: anthropic.String(*agentID)}
}

func deploymentScheduleToParam(s DeploymentSchedule) anthropic.BetaManagedAgentsScheduleParams {
	params := anthropic.BetaManagedAgentsScheduleParams{
		Type: anthropic.BetaManagedAgentsScheduleParamsTypeCron,
	}
	if s.Expression != nil {
		params.Expression = *s.Expression
	}
	if s.Timezone != nil {
		params.Timezone = *s.Timezone
	}
	return params
}

func deploymentScheduleFromObservation(s anthropic.BetaManagedAgentsSchedule) *DeploymentScheduleObservation {
	obs := &DeploymentScheduleObservation{
		Expression: &s.Expression,
		Timezone:   &s.Timezone,
	}
	if !s.LastRunAt.IsZero() {
		lastRun := s.LastRunAt.Format(time.RFC3339)
		obs.LastRunAt = &lastRun
	}
	for _, t := range s.UpcomingRunsAt {
		obs.UpcomingRunsAt = append(obs.UpcomingRunsAt, t.Format(time.RFC3339))
	}
	return obs
}

func deploymentInitialEventToParam(ev DeploymentInitialEvent) anthropic.BetaManagedAgentsDeploymentInitialEventParamsUnion {
	evType := ""
	if ev.Type != nil {
		evType = *ev.Type
	}
	switch evType {
	case "system.message":
		sys := &anthropic.BetaManagedAgentsSystemMessageEventParams{
			Type: anthropic.BetaManagedAgentsSystemMessageEventParamsTypeSystemMessage,
		}
		for _, cb := range ev.Content {
			text := ""
			if cb.Text != nil {
				text = *cb.Text
			}
			sys.Content = append(sys.Content, anthropic.BetaManagedAgentsSystemContentBlockParam{
				Text: text,
				Type: anthropic.BetaManagedAgentsSystemContentBlockTypeText,
			})
		}
		return anthropic.BetaManagedAgentsDeploymentInitialEventParamsUnion{OfSystemMessage: sys}

	case "user.message":
		um := &anthropic.BetaManagedAgentsUserMessageEventParams{
			Type: anthropic.BetaManagedAgentsUserMessageEventParamsTypeUserMessage,
		}
		for _, cb := range ev.Content {
			um.Content = append(um.Content, deploymentContentBlockToParam(cb))
		}
		return anthropic.BetaManagedAgentsDeploymentInitialEventParamsUnion{OfUserMessage: um}

	case "user.define_outcome":
		od := &anthropic.BetaManagedAgentsUserDefineOutcomeEventParams{
			Type: anthropic.BetaManagedAgentsUserDefineOutcomeEventParamsTypeUserDefineOutcome,
		}
		if ev.Description != nil {
			od.Description = *ev.Description
		}
		if ev.MaxIterations != nil {
			od.MaxIterations = anthropic.Int(*ev.MaxIterations)
		}
		if ev.Rubric != nil {
			od.Rubric = deploymentRubricToParam(*ev.Rubric)
		}
		return anthropic.BetaManagedAgentsDeploymentInitialEventParamsUnion{OfUserDefineOutcome: od}
	}
	return anthropic.BetaManagedAgentsDeploymentInitialEventParamsUnion{}
}

func deploymentContentBlockToParam(cb DeploymentContentBlock) anthropic.BetaManagedAgentsUserMessageEventParamsContentUnion {
	cbType := ""
	if cb.Type != nil {
		cbType = *cb.Type
	}
	switch cbType {
	case "text":
		text := ""
		if cb.Text != nil {
			text = *cb.Text
		}
		return anthropic.BetaManagedAgentsUserMessageEventParamsContentUnion{
			OfText: &anthropic.BetaManagedAgentsTextBlockParam{
				Text: text,
				Type: anthropic.BetaManagedAgentsTextBlockTypeText,
			},
		}
	case "image":
		img := &anthropic.BetaManagedAgentsImageBlockParam{
			Type: anthropic.BetaManagedAgentsImageBlockTypeImage,
		}
		if cb.Source != nil {
			img.Source = deploymentImageSourceToParam(*cb.Source)
		}
		return anthropic.BetaManagedAgentsUserMessageEventParamsContentUnion{OfImage: img}
	case "document":
		doc := &anthropic.BetaManagedAgentsDocumentBlockParam{
			Type: anthropic.BetaManagedAgentsDocumentBlockTypeDocument,
		}
		if cb.Source != nil {
			doc.Source = deploymentDocumentSourceToParam(*cb.Source)
		}
		if cb.Context != nil {
			doc.Context = anthropic.String(*cb.Context)
		}
		if cb.Title != nil {
			doc.Title = anthropic.String(*cb.Title)
		}
		return anthropic.BetaManagedAgentsUserMessageEventParamsContentUnion{OfDocument: doc}
	}
	return anthropic.BetaManagedAgentsUserMessageEventParamsContentUnion{}
}

func deploymentImageSourceToParam(s DeploymentBlockSource) anthropic.BetaManagedAgentsImageBlockSourceUnionParam {
	srcType := ""
	if s.Type != nil {
		srcType = *s.Type
	}
	switch srcType {
	case "base64":
		src := &anthropic.BetaManagedAgentsBase64ImageSourceParam{
			Type: anthropic.BetaManagedAgentsBase64ImageSourceTypeBase64,
		}
		if s.Data != nil {
			src.Data = *s.Data
		}
		if s.MediaType != nil {
			src.MediaType = *s.MediaType
		}
		return anthropic.BetaManagedAgentsImageBlockSourceUnionParam{OfBase64: src}
	case "url":
		src := &anthropic.BetaManagedAgentsURLImageSourceParam{
			Type: anthropic.BetaManagedAgentsURLImageSourceTypeURL,
		}
		if s.URL != nil {
			src.URL = *s.URL
		}
		return anthropic.BetaManagedAgentsImageBlockSourceUnionParam{OfURL: src}
	case "file":
		src := &anthropic.BetaManagedAgentsFileImageSourceParam{
			Type: anthropic.BetaManagedAgentsFileImageSourceTypeFile,
		}
		if s.FileID != nil {
			src.FileID = *s.FileID
		}
		return anthropic.BetaManagedAgentsImageBlockSourceUnionParam{OfFile: src}
	}
	return anthropic.BetaManagedAgentsImageBlockSourceUnionParam{}
}

func deploymentDocumentSourceToParam(s DeploymentBlockSource) anthropic.BetaManagedAgentsDocumentBlockSourceUnionParam {
	srcType := ""
	if s.Type != nil {
		srcType = *s.Type
	}
	switch srcType {
	case "base64":
		src := &anthropic.BetaManagedAgentsBase64DocumentSourceParam{
			Type: anthropic.BetaManagedAgentsBase64DocumentSourceTypeBase64,
		}
		if s.Data != nil {
			src.Data = *s.Data
		}
		if s.MediaType != nil {
			src.MediaType = *s.MediaType
		}
		return anthropic.BetaManagedAgentsDocumentBlockSourceUnionParam{OfBase64: src}
	case "text":
		src := &anthropic.BetaManagedAgentsPlainTextDocumentSourceParam{
			Type:      anthropic.BetaManagedAgentsPlainTextDocumentSourceTypeText,
			MediaType: anthropic.BetaManagedAgentsPlainTextDocumentSourceMediaTypeTextPlain,
		}
		if s.Data != nil {
			src.Data = *s.Data
		}
		return anthropic.BetaManagedAgentsDocumentBlockSourceUnionParam{OfText: src}
	case "url":
		src := &anthropic.BetaManagedAgentsURLDocumentSourceParam{
			Type: anthropic.BetaManagedAgentsURLDocumentSourceTypeURL,
		}
		if s.URL != nil {
			src.URL = *s.URL
		}
		return anthropic.BetaManagedAgentsDocumentBlockSourceUnionParam{OfURL: src}
	case "file":
		src := &anthropic.BetaManagedAgentsFileDocumentSourceParam{
			Type: anthropic.BetaManagedAgentsFileDocumentSourceTypeFile,
		}
		if s.FileID != nil {
			src.FileID = *s.FileID
		}
		return anthropic.BetaManagedAgentsDocumentBlockSourceUnionParam{OfFile: src}
	}
	return anthropic.BetaManagedAgentsDocumentBlockSourceUnionParam{}
}

func deploymentRubricToParam(r DeploymentRubric) anthropic.BetaManagedAgentsUserDefineOutcomeEventParamsRubricUnion {
	rType := ""
	if r.Type != nil {
		rType = *r.Type
	}
	switch rType {
	case "text":
		rubric := &anthropic.BetaManagedAgentsTextRubricParams{
			Type: anthropic.BetaManagedAgentsTextRubricParamsTypeText,
		}
		if r.Content != nil {
			rubric.Content = *r.Content
		}
		return anthropic.BetaManagedAgentsUserDefineOutcomeEventParamsRubricUnion{OfText: rubric}
	case "file":
		rubric := &anthropic.BetaManagedAgentsFileRubricParams{
			Type: anthropic.BetaManagedAgentsFileRubricParamsTypeFile,
		}
		if r.FileID != nil {
			rubric.FileID = *r.FileID
		}
		return anthropic.BetaManagedAgentsUserDefineOutcomeEventParamsRubricUnion{OfFile: rubric}
	}
	return anthropic.BetaManagedAgentsUserDefineOutcomeEventParamsRubricUnion{}
}

func deploymentResourceToNewParam(res SessionResource, authToken string) anthropic.BetaDeploymentNewParamsResourceUnion {
	switch deploymentResourceType(res) {
	case "github_repository":
		gh := deploymentGitHubResourceParams(res, authToken)
		return anthropic.BetaDeploymentNewParamsResourceUnion{OfGitHubRepository: gh}
	case "file":
		return anthropic.BetaDeploymentNewParamsResourceUnion{OfFile: deploymentFileResourceParams(res)}
	case "memory_store":
		return anthropic.BetaDeploymentNewParamsResourceUnion{OfMemoryStore: deploymentMemoryStoreResourceParams(res)}
	}
	return anthropic.BetaDeploymentNewParamsResourceUnion{}
}

func deploymentResourceToUpdateParam(res SessionResource, authToken string) anthropic.BetaDeploymentUpdateParamsResourceUnion {
	switch deploymentResourceType(res) {
	case "github_repository":
		gh := deploymentGitHubResourceParams(res, authToken)
		return anthropic.BetaDeploymentUpdateParamsResourceUnion{OfGitHubRepository: gh}
	case "file":
		return anthropic.BetaDeploymentUpdateParamsResourceUnion{OfFile: deploymentFileResourceParams(res)}
	case "memory_store":
		return anthropic.BetaDeploymentUpdateParamsResourceUnion{OfMemoryStore: deploymentMemoryStoreResourceParams(res)}
	}
	return anthropic.BetaDeploymentUpdateParamsResourceUnion{}
}

func deploymentResourceType(res SessionResource) string {
	if res.Type != nil {
		return *res.Type
	}
	return ""
}

func deploymentGitHubResourceParams(res SessionResource, authToken string) *anthropic.BetaManagedAgentsGitHubRepositoryResourceParams {
	gh := &anthropic.BetaManagedAgentsGitHubRepositoryResourceParams{
		Type: anthropic.BetaManagedAgentsGitHubRepositoryResourceParamsTypeGitHubRepository,
	}
	if res.URL != nil {
		gh.URL = *res.URL
	}
	if authToken != "" {
		gh.AuthorizationToken = authToken
	}
	if res.MountPath != nil {
		gh.MountPath = anthropic.String(*res.MountPath)
	}
	if res.Checkout != nil {
		gh.Checkout = deploymentCheckoutUnion(*res.Checkout)
	}
	return gh
}

func deploymentCheckoutUnion(c SessionResourceCheckout) anthropic.BetaManagedAgentsGitHubRepositoryResourceParamsCheckoutUnion {
	checkoutType := ""
	if c.Type != nil {
		checkoutType = *c.Type
	}
	switch checkoutType {
	case "branch":
		if c.Name != nil {
			return anthropic.BetaManagedAgentsGitHubRepositoryResourceParamsCheckoutUnion{
				OfBranch: &anthropic.BetaManagedAgentsBranchCheckoutParam{
					Name: *c.Name,
					Type: anthropic.BetaManagedAgentsBranchCheckoutTypeBranch,
				},
			}
		}
	case "commit":
		if c.Sha != nil {
			return anthropic.BetaManagedAgentsGitHubRepositoryResourceParamsCheckoutUnion{
				OfCommit: &anthropic.BetaManagedAgentsCommitCheckoutParam{
					Sha:  *c.Sha,
					Type: anthropic.BetaManagedAgentsCommitCheckoutTypeCommit,
				},
			}
		}
	}
	return anthropic.BetaManagedAgentsGitHubRepositoryResourceParamsCheckoutUnion{}
}

func deploymentFileResourceParams(res SessionResource) *anthropic.BetaManagedAgentsFileResourceParams {
	fileParams := &anthropic.BetaManagedAgentsFileResourceParams{
		Type: anthropic.BetaManagedAgentsFileResourceParamsTypeFile,
	}
	if res.FileID != nil {
		fileParams.FileID = *res.FileID
	}
	if res.MountPath != nil {
		fileParams.MountPath = anthropic.String(*res.MountPath)
	}
	return fileParams
}

func deploymentMemoryStoreResourceParams(res SessionResource) *anthropic.BetaManagedAgentsMemoryStoreResourceParam {
	msParams := &anthropic.BetaManagedAgentsMemoryStoreResourceParam{
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
	return msParams
}

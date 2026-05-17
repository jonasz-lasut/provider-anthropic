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

// Package agent implements the Crossplane managed reconciler for the Anthropic
// Managed Agents beta API.
package agent

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
	errNotAgent  = "managed resource is not an Agent"
	errNewClient = "cannot build Anthropic client"
	errObserve   = "cannot observe Agent"
	errCreate    = "cannot create Agent"
	errUpdate    = "cannot update Agent"
	errDelete    = "cannot archive Agent"
)

// Setup adds a controller for Agent to the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(betav1alpha1.AgentKind)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&betav1alpha1.Agent{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(betav1alpha1.AgentGroupVersionKind),
			managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
			managed.WithLogger(o.Logger.WithValues("controller", name)),
			managed.WithPollInterval(o.PollInterval),
			managed.WithManagementPolicies(),
		))
}

// SetupGated registers the Agent controller to start only once the Agent CRD is established.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(err)
		}
	}, betav1alpha1.AgentGroupVersionKind)
	return nil
}

// connector builds an ExternalClient for each reconcile.
type connector struct {
	kube client.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	ag, ok := mg.(*betav1alpha1.Agent)
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
	ag, ok := mg.(*betav1alpha1.Agent)
	if !ok {
		return managed.ExternalObservation{}, xperrors.New(errNotAgent)
	}

	// The external-name annotation holds the Anthropic agent ID once created.
	// If it still equals the k8s object name the resource has never been created.
	agentID := meta.GetExternalName(ag)
	if agentID == ag.GetName() {
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

	// Populate observed state.
	ag.Status.AtProvider.ID = &resp.ID
	createdAt := resp.CreatedAt.String()
	updatedAt := resp.UpdatedAt.String()
	ag.Status.AtProvider.CreatedAt = &createdAt
	ag.Status.AtProvider.UpdatedAt = &updatedAt
	ag.Status.AtProvider.Version = &resp.Version

	ag.SetConditions(xpv1.Available())

	system, err := clients.ResolveLocalSecretKey(ctx, e.kube, ag.Spec.ForProvider.SystemSecretRef, ag.GetNamespace())
	if err != nil {
		return managed.ExternalObservation{}, xperrors.Wrap(err, errObserve)
	}
	upToDate := isUpToDate(ag, resp, system)
	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate,
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	ag, ok := mg.(*betav1alpha1.Agent)
	if !ok {
		return managed.ExternalCreation{}, xperrors.New(errNotAgent)
	}

	system, err := clients.ResolveLocalSecretKey(ctx, e.kube, ag.Spec.ForProvider.SystemSecretRef, ag.GetNamespace())
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}
	params := buildNewParams(ag.Spec.ForProvider, system)
	resp, err := e.client.Beta.Agents.New(ctx, params)
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}

	// Store ID in external-name (primary key for the reconciler) and mirror
	// it in AtProvider so cross-resource references can extract it via
	// ComputedFieldExtractor("id").
	meta.SetExternalName(ag, resp.ID)
	ag.Status.AtProvider.ID = &resp.ID

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	ag, ok := mg.(*betav1alpha1.Agent)
	if !ok {
		return managed.ExternalUpdate{}, xperrors.New(errNotAgent)
	}

	agentID := meta.GetExternalName(ag)
	if agentID == ag.GetName() {
		return managed.ExternalUpdate{}, xperrors.New("external name not yet set; skipping update")
	}

	if ag.Status.AtProvider.Version == nil {
		return managed.ExternalUpdate{}, xperrors.New("version not yet observed; skipping update")
	}

	system, err := clients.ResolveLocalSecretKey(ctx, e.kube, ag.Spec.ForProvider.SystemSecretRef, ag.GetNamespace())
	if err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}
	params := buildUpdateParams(ag.Spec.ForProvider, *ag.Status.AtProvider.Version, system)
	if _, err := e.client.Beta.Agents.Update(ctx, agentID, params); err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	ag, ok := mg.(*betav1alpha1.Agent)
	if !ok {
		return managed.ExternalDelete{}, xperrors.New(errNotAgent)
	}

	agentID := meta.GetExternalName(ag)
	if agentID == ag.GetName() {
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

// buildNewParams converts ForProvider into the SDK create params.
func buildNewParams(p betav1alpha1.AgentParameters, system string) anthropic.BetaAgentNewParams {
	params := anthropic.BetaAgentNewParams{}

	if p.Name != nil {
		params.Name = *p.Name
	}
	if p.Model != nil {
		params.Model = anthropic.BetaManagedAgentsModelConfigParams{ID: *p.Model}
	}

	if p.Description != nil {
		params.Description = anthropic.String(*p.Description)
	}
	if system != "" {
		params.System = anthropic.String(system)
	}
	if p.Metadata != nil {
		params.Metadata = p.Metadata
	}

	for _, s := range p.MCPServers {
		srv := anthropic.BetaManagedAgentsURLMCPServerParams{
			Type: anthropic.BetaManagedAgentsURLMCPServerParamsTypeURL,
		}
		if s.Name != nil {
			srv.Name = *s.Name
		}
		if s.URL != nil {
			srv.URL = *s.URL
		}
		params.MCPServers = append(params.MCPServers, srv)
	}

	for _, sk := range p.Skills {
		union := skillToParam(sk)
		params.Skills = append(params.Skills, union)
	}

	for _, t := range p.Tools {
		union := toolToNewParam(t)
		params.Tools = append(params.Tools, union)
	}

	return params
}

// buildUpdateParams converts ForProvider into the SDK update params.
// version is the optimistic-concurrency token required by the API.
func buildUpdateParams(p betav1alpha1.AgentParameters, version int64, system string) anthropic.BetaAgentUpdateParams {
	params := anthropic.BetaAgentUpdateParams{
		Version: version,
	}

	if p.Name != nil {
		params.Name = anthropic.String(*p.Name)
	}
	if p.Model != nil {
		params.Model = anthropic.BetaManagedAgentsModelConfigParams{ID: *p.Model}
	}

	if p.Description != nil {
		params.Description = anthropic.String(*p.Description)
	}
	if system != "" {
		params.System = anthropic.String(system)
	}
	if p.Metadata != nil {
		params.Metadata = p.Metadata
	}

	for _, s := range p.MCPServers {
		srv := anthropic.BetaManagedAgentsURLMCPServerParams{
			Type: anthropic.BetaManagedAgentsURLMCPServerParamsTypeURL,
		}
		if s.Name != nil {
			srv.Name = *s.Name
		}
		if s.URL != nil {
			srv.URL = *s.URL
		}
		params.MCPServers = append(params.MCPServers, srv)
	}

	for _, sk := range p.Skills {
		union := skillToParam(sk)
		params.Skills = append(params.Skills, union)
	}

	for _, t := range p.Tools {
		union := toolToUpdateParam(t)
		params.Tools = append(params.Tools, union)
	}

	return params
}

func skillToParam(s betav1alpha1.AgentSkillConfig) anthropic.BetaManagedAgentsSkillParamsUnion {
	skillID := ""
	if s.SkillID != nil {
		skillID = *s.SkillID
	}
	skillType := ""
	if s.Type != nil {
		skillType = *s.Type
	}
	switch skillType {
	case "anthropic":
		return anthropic.BetaManagedAgentsSkillParamsUnion{
			OfAnthropic: &anthropic.BetaManagedAgentsAnthropicSkillParams{
				SkillID: skillID,
				Type:    anthropic.BetaManagedAgentsAnthropicSkillParamsTypeAnthropic,
			},
		}
	default: // "custom"
		return anthropic.BetaManagedAgentsSkillParamsUnion{
			OfCustom: &anthropic.BetaManagedAgentsCustomSkillParams{
				SkillID: skillID,
				Type:    anthropic.BetaManagedAgentsCustomSkillParamsTypeCustom,
			},
		}
	}
}

func toolToNewParam(_ betav1alpha1.AgentToolConfig) anthropic.BetaAgentNewParamsToolUnion {
	return anthropic.BetaAgentNewParamsToolUnion{
		OfAgentToolset20260401: &anthropic.BetaManagedAgentsAgentToolset20260401Params{
			Type: anthropic.BetaManagedAgentsAgentToolset20260401ParamsTypeAgentToolset20260401,
		},
	}
}

func toolToUpdateParam(_ betav1alpha1.AgentToolConfig) anthropic.BetaAgentUpdateParamsToolUnion {
	return anthropic.BetaAgentUpdateParamsToolUnion{
		OfAgentToolset20260401: &anthropic.BetaManagedAgentsAgentToolset20260401Params{
			Type: anthropic.BetaManagedAgentsAgentToolset20260401ParamsTypeAgentToolset20260401,
		},
	}
}

// isUpToDate compares the desired state with the observed agent.
// system is the resolved system-prompt string (from SystemSecretRef);
// an empty value means "no Secret content provided" and the field is
// not diffed. The Agent API returns the full system prompt back in
// resp.System, so we string-compare directly — no hash is available on
// this resource (unlike MemoryStoreMemory.ContentSha256).
func isUpToDate(ag *betav1alpha1.Agent, resp *anthropic.BetaManagedAgentsAgent, system string) bool {
	p := ag.Spec.ForProvider

	if p.Name != nil && *p.Name != resp.Name {
		return false
	}
	if p.Model != nil && resp.Model.ID != *p.Model {
		return false
	}
	if p.Description != nil && *p.Description != resp.Description {
		return false
	}
	if system != "" && system != resp.System {
		return false
	}

	// MCPServers
	if len(p.MCPServers) != len(resp.MCPServers) {
		return false
	}
	for i, s := range p.MCPServers {
		if s.Name != nil && *s.Name != resp.MCPServers[i].Name {
			return false
		}
		if s.URL != nil && *s.URL != resp.MCPServers[i].URL {
			return false
		}
	}

	// Skills
	if len(p.Skills) != len(resp.Skills) {
		return false
	}

	// Tools
	if len(p.Tools) != len(resp.Tools) {
		return false
	}

	// Metadata
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

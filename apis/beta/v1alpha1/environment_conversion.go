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
	anthropic "github.com/anthropics/anthropic-sdk-go"
)

func (r *Environment) ToAnthropicNew() anthropic.BetaEnvironmentNewParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaEnvironmentNewParams{}
	if p.Name != nil {
		params.Name = *p.Name
	}
	if p.Description != nil {
		params.Description = anthropic.String(*p.Description)
	}
	if p.Metadata != nil {
		params.Metadata = p.Metadata
	}
	if p.Config != nil {
		cfg := envCloudConfigToParam(p.Config)
		params.Config = anthropic.BetaEnvironmentNewParamsConfigUnion{OfCloud: &cfg}
	}
	if p.Scope != nil {
		params.Scope = anthropic.BetaEnvironmentNewParamsScope(*p.Scope)
	}
	return params
}

func (r *Environment) ToAnthropicUpdate() anthropic.BetaEnvironmentUpdateParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaEnvironmentUpdateParams{}
	if p.Name != nil {
		params.Name = anthropic.String(*p.Name)
	}
	if p.Description != nil {
		params.Description = anthropic.String(*p.Description)
	}
	if p.Metadata != nil {
		params.Metadata = p.Metadata
	}
	if p.Config != nil {
		cfg := envCloudConfigToParam(p.Config)
		params.Config = anthropic.BetaEnvironmentUpdateParamsConfigUnion{OfCloud: &cfg}
	}
	if p.Scope != nil {
		params.Scope = anthropic.BetaEnvironmentUpdateParamsScope(*p.Scope)
	}
	return params
}

func (r *Environment) FromAnthropicObservation(resp anthropic.BetaEnvironment) {
	r.Status.AtProvider.ID = &resp.ID
	r.Status.AtProvider.Name = &resp.Name
	r.Status.AtProvider.Description = &resp.Description
	r.Status.AtProvider.Metadata = resp.Metadata
	r.Status.AtProvider.CreatedAt = &resp.CreatedAt
	r.Status.AtProvider.UpdatedAt = &resp.UpdatedAt
	// ArchivedAt intentionally omitted
	if scope := string(resp.Scope); scope != "" {
		r.Status.AtProvider.Scope = &scope
	}
	netType := resp.Config.Networking.Type
	net := &EnvironmentNetworkingConfig{Type: &netType}
	if netType == "limited" {
		allowMCP := resp.Config.Networking.AllowMCPServers
		allowPkg := resp.Config.Networking.AllowPackageManagers
		net.AllowMCPServers = &allowMCP
		net.AllowPackageManagers = &allowPkg
		net.AllowedHosts = resp.Config.Networking.AllowedHosts
	}
	pkgs := resp.Config.Packages
	r.Status.AtProvider.Config = &EnvironmentCloudConfig{
		Networking: net,
		Packages: &EnvironmentPackages{
			Apt:   pkgs.Apt,
			Cargo: pkgs.Cargo,
			Gem:   pkgs.Gem,
			Go:    pkgs.Go,
			Npm:   pkgs.Npm,
			Pip:   pkgs.Pip,
		},
	}
}

func envCloudConfigToParam(cfg *EnvironmentCloudConfig) anthropic.BetaCloudConfigParams {
	params := anthropic.BetaCloudConfigParams{}
	if cfg.Networking != nil {
		netType := ""
		if cfg.Networking.Type != nil {
			netType = *cfg.Networking.Type
		}
		switch netType {
		case "unrestricted":
			unr := anthropic.NewBetaUnrestrictedNetworkParam()
			params.Networking = anthropic.BetaCloudConfigParamsNetworkingUnion{
				OfUnrestricted: &unr,
			}
		case "limited":
			limited := anthropic.BetaLimitedNetworkParams{
				Type:         "limited",
				AllowedHosts: cfg.Networking.AllowedHosts,
			}
			if cfg.Networking.AllowMCPServers != nil {
				limited.AllowMCPServers = anthropic.Bool(*cfg.Networking.AllowMCPServers)
			}
			if cfg.Networking.AllowPackageManagers != nil {
				limited.AllowPackageManagers = anthropic.Bool(*cfg.Networking.AllowPackageManagers)
			}
			params.Networking = anthropic.BetaCloudConfigParamsNetworkingUnion{
				OfLimited: &limited,
			}
		}
	}
	if cfg.Packages != nil {
		params.Packages = anthropic.BetaPackagesParams{
			Apt:   cfg.Packages.Apt,
			Cargo: cfg.Packages.Cargo,
			Gem:   cfg.Packages.Gem,
			Go:    cfg.Packages.Go,
			Npm:   cfg.Packages.Npm,
			Pip:   cfg.Packages.Pip,
		}
	}
	return params
}

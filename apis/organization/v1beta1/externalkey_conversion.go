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

// ToAnthropicNew converts ForProvider to BetaOrganizationExternalKeyNewParams.
func (r *ExternalKey) ToAnthropicNew() anthropic.BetaOrganizationExternalKeyNewParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaOrganizationExternalKeyNewParams{}
	if p.DisplayName != nil {
		params.DisplayName = anthropic.String(*p.DisplayName)
	}
	if p.Geo != nil {
		params.Geo = anthropic.BetaOrganizationExternalKeyNewParamsGeo(*p.Geo)
	}
	if pc := p.ProviderConfig; pc != nil && pc.Type != nil {
		switch *pc.Type {
		case "aws":
			params.ProviderConfig.OfAWS = awsExternalKeyConfig(pc)
		case "gcp":
			params.ProviderConfig.OfGCP = gcpExternalKeyConfig(pc)
		case "azure":
			params.ProviderConfig.OfAzure = azureExternalKeyConfig(pc)
		}
	}
	return params
}

// ToAnthropicUpdate converts ForProvider to BetaOrganizationExternalKeyUpdateParams.
func (r *ExternalKey) ToAnthropicUpdate() anthropic.BetaOrganizationExternalKeyUpdateParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaOrganizationExternalKeyUpdateParams{}
	if p.DisplayName != nil {
		params.DisplayName = anthropic.String(*p.DisplayName)
	}
	if p.Geo != nil {
		params.Geo = anthropic.BetaOrganizationExternalKeyUpdateParamsGeo(*p.Geo)
	}
	if pc := p.ProviderConfig; pc != nil && pc.Type != nil {
		switch *pc.Type {
		case "aws":
			params.ProviderConfig.OfAWS = awsExternalKeyConfig(pc)
		case "gcp":
			params.ProviderConfig.OfGCP = gcpExternalKeyConfig(pc)
		case "azure":
			params.ProviderConfig.OfAzure = azureExternalKeyConfig(pc)
		}
	}
	return params
}

func awsExternalKeyConfig(pc *ExternalKeyProviderConfig) *anthropic.BetaAWSExternalKeyConfigParam {
	cfg := &anthropic.BetaAWSExternalKeyConfigParam{}
	if pc.KMSARN != nil {
		cfg.KMSARN = *pc.KMSARN
	}
	if pc.Region != nil {
		cfg.Region = anthropic.String(*pc.Region)
	}
	return cfg
}

func gcpExternalKeyConfig(pc *ExternalKeyProviderConfig) *anthropic.BetaGCPExternalKeyConfigParam {
	cfg := &anthropic.BetaGCPExternalKeyConfigParam{}
	if pc.KeyName != nil {
		cfg.KeyName = *pc.KeyName
	}
	return cfg
}

func azureExternalKeyConfig(pc *ExternalKeyProviderConfig) *anthropic.BetaAzureExternalKeyConfigParam {
	cfg := &anthropic.BetaAzureExternalKeyConfigParam{}
	if pc.KeyName != nil {
		cfg.KeyName = *pc.KeyName
	}
	if pc.TenantID != nil {
		cfg.TenantID = *pc.TenantID
	}
	if pc.VaultURI != nil {
		cfg.VaultURI = *pc.VaultURI
	}
	if pc.ClientID != nil {
		cfg.ClientID = anthropic.String(*pc.ClientID)
	}
	return cfg
}

// FromAnthropicObservation populates AtProvider from a BetaExternalKey.
func (r *ExternalKey) FromAnthropicObservation(resp anthropic.BetaExternalKey) {
	r.Status.AtProvider.ID = &resp.ID
	r.Status.AtProvider.DisplayName = &resp.DisplayName
	r.Status.AtProvider.Geo = &resp.Geo
	r.Status.AtProvider.ProviderConfig = externalKeyProviderConfigObservation(resp.ProviderConfig)
	r.Status.AtProvider.Attachment = nil
	if resp.Attachment.Type != "" {
		r.Status.AtProvider.Attachment = &resp.Attachment.Type
	}
	createdAt := resp.CreatedAt.Format(time.RFC3339)
	r.Status.AtProvider.CreatedAt = &createdAt
	updatedAt := resp.UpdatedAt.Format(time.RFC3339)
	r.Status.AtProvider.UpdatedAt = &updatedAt
}

func externalKeyProviderConfigObservation(u anthropic.BetaExternalKeyProviderConfigUnion) *ExternalKeyProviderConfig {
	out := &ExternalKeyProviderConfig{}
	set := func(dst **string, v string) {
		if v != "" {
			*dst = &v
		}
	}
	set(&out.Type, u.Type)
	set(&out.KMSARN, u.KMSARN)
	set(&out.Region, u.Region)
	set(&out.KeyName, u.KeyName)
	set(&out.TenantID, u.TenantID)
	set(&out.VaultURI, u.VaultURI)
	set(&out.ClientID, u.ClientID)
	return out
}

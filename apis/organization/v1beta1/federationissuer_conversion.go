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
	"encoding/json"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// ToAnthropicNew converts ForProvider to BetaOrganizationFederationIssuerNewParams.
func (r *FederationIssuer) ToAnthropicNew() anthropic.BetaOrganizationFederationIssuerNewParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaOrganizationFederationIssuerNewParams{}
	if p.IssuerURL != nil {
		params.IssuerURL = *p.IssuerURL
	}
	if p.Name != nil {
		params.Name = *p.Name
	}
	if p.CheckJTI != nil {
		params.CheckJTI = anthropic.Bool(*p.CheckJTI)
	}
	if p.MaxJWTLifetimeSeconds != nil {
		params.MaxJWTLifetimeSeconds = anthropic.Int(*p.MaxJWTLifetimeSeconds)
	}
	if j := p.JWKS; j != nil && j.Type != nil {
		switch *j.Type {
		case "discovery":
			params.JWKS.OfDiscovery = jwksDiscoveryParam(j)
		case "explicit_url":
			params.JWKS.OfExplicitURL = jwksExplicitURLParam(j)
		case "inline":
			params.JWKS.OfInline = jwksInlineParam(j)
		}
	}
	return params
}

// ToAnthropicUpdate converts ForProvider to BetaOrganizationFederationIssuerUpdateParams.
func (r *FederationIssuer) ToAnthropicUpdate() anthropic.BetaOrganizationFederationIssuerUpdateParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaOrganizationFederationIssuerUpdateParams{}
	if p.IssuerURL != nil {
		params.IssuerURL = anthropic.String(*p.IssuerURL)
	}
	if p.Name != nil {
		params.Name = anthropic.String(*p.Name)
	}
	if p.CheckJTI != nil {
		params.CheckJTI = anthropic.Bool(*p.CheckJTI)
	}
	if p.MaxJWTLifetimeSeconds != nil {
		params.MaxJWTLifetimeSeconds = anthropic.Int(*p.MaxJWTLifetimeSeconds)
	}
	if p.JWKSPollingDisabled != nil {
		params.JWKSPollingDisabled = anthropic.Bool(*p.JWKSPollingDisabled)
	}
	if j := p.JWKS; j != nil && j.Type != nil {
		switch *j.Type {
		case "discovery":
			params.JWKS.OfDiscovery = jwksDiscoveryParam(j)
		case "explicit_url":
			params.JWKS.OfExplicitURL = jwksExplicitURLParam(j)
		case "inline":
			params.JWKS.OfInline = jwksInlineParam(j)
		}
	}
	return params
}

func jwksDiscoveryParam(j *FederationIssuerJWKS) *anthropic.BetaJWKSDiscoveryParam {
	cfg := &anthropic.BetaJWKSDiscoveryParam{}
	if j.DiscoveryBase != nil {
		cfg.DiscoveryBase = anthropic.String(*j.DiscoveryBase)
	}
	if j.CACertPEM != nil {
		cfg.CACertPEM = anthropic.String(*j.CACertPEM)
	}
	return cfg
}

func jwksExplicitURLParam(j *FederationIssuerJWKS) *anthropic.BetaJWKSExplicitURLParam {
	cfg := &anthropic.BetaJWKSExplicitURLParam{}
	if j.URL != nil {
		cfg.URL = *j.URL
	}
	if j.CACertPEM != nil {
		cfg.CACertPEM = anthropic.String(*j.CACertPEM)
	}
	return cfg
}

func jwksInlineParam(j *FederationIssuerJWKS) *anthropic.BetaJWKSInlineParam {
	return &anthropic.BetaJWKSInlineParam{Keys: jwksKeysToSDK(j.Keys)}
}

// jwksKeysToSDK decodes the JWK objects held as raw JSON in the CRD. Entries
// that are not JSON objects are skipped; the API server has already rejected
// anything that is not valid JSON.
func jwksKeysToSDK(keys []apiextensionsv1.JSON) []map[string]any {
	out := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		var m map[string]any
		if err := json.Unmarshal(k.Raw, &m); err != nil || m == nil {
			continue
		}
		out = append(out, m)
	}
	return out
}

func jwksKeysFromSDK(keys []map[string]any) []apiextensionsv1.JSON {
	if len(keys) == 0 {
		return nil
	}
	out := make([]apiextensionsv1.JSON, 0, len(keys))
	for _, k := range keys {
		raw, err := json.Marshal(k)
		if err != nil {
			continue
		}
		out = append(out, apiextensionsv1.JSON{Raw: raw})
	}
	return out
}

// FromAnthropicObservation populates AtProvider from a BetaFederationIssuer.
// ArchivedAt is intentionally omitted: the reconciler treats an archived
// issuer as absent.
func (r *FederationIssuer) FromAnthropicObservation(resp anthropic.BetaFederationIssuer) {
	r.Status.AtProvider.ID = &resp.ID
	r.Status.AtProvider.Name = &resp.Name
	r.Status.AtProvider.IssuerURL = &resp.IssuerURL
	checkJTI := resp.CheckJTI
	r.Status.AtProvider.CheckJTI = &checkJTI
	maxLifetime := resp.MaxJWTLifetimeSeconds
	r.Status.AtProvider.MaxJWTLifetimeSeconds = &maxLifetime
	r.Status.AtProvider.JWKS = &FederationIssuerJWKS{
		Type:          optionalString(resp.JWKS.Type),
		DiscoveryBase: optionalString(resp.JWKS.DiscoveryBase),
		URL:           optionalString(resp.JWKS.URL),
		CACertPEM:     optionalString(resp.JWKS.CACertPEM),
		Keys:          jwksKeysFromSDK(resp.JWKS.Keys),
	}
	r.Status.AtProvider.JWKSPollingDisabledAt = optionalTime(resp.JWKSPollingDisabledAt)
	failures := resp.PollStatus.ConsecutiveFailures
	r.Status.AtProvider.PollStatus = &FederationIssuerPollStatus{
		ConsecutiveFailures: &failures,
		LastFetchedAt:       optionalTime(resp.PollStatus.LastFetchedAt),
		NextPollAt:          optionalTime(resp.PollStatus.NextPollAt),
	}
	r.Status.AtProvider.CreatedAt = optionalTime(resp.CreatedAt)
	r.Status.AtProvider.UpdatedAt = optionalTime(resp.UpdatedAt)
	r.Status.AtProvider.CreatedByActorID = optionalString(resp.CreatedByActorID)
	r.Status.AtProvider.UpdatedByActorID = optionalString(resp.UpdatedByActorID)
}

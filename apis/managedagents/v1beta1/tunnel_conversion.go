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
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
)

// TunnelConversionContext carries the tunnel's connection-detail values so the
// reconciler can publish them as Crossplane connection details: the
// Anthropic-assigned domain plus the revealed connector token.
//
// Unlike other resources' contexts (which carry resolved *input* secrets read
// from Kubernetes), these are *outputs* fetched live from the API — the domain
// from the Get/New response and the token from Beta.Tunnels.RevealToken — on
// every Observe and on Create.
type TunnelConversionContext struct {
	// Domain is the Anthropic-assigned hostname for the tunnel. Also surfaced in
	// status.atProvider.domain; published here so consumers receive it alongside
	// the connector token.
	Domain string
	// TunnelToken is the connector token used to run the tunnel; treat as a
	// credential.
	TunnelToken string
	// TokenID is the stable identifier of the current token value; it changes
	// when the token is rotated.
	TokenID string
}

// ToConnectionDetails publishes the tunnel's domain and revealed token as
// Crossplane connection details so consumers can access them via
// spec.writeConnectionSecretToRef.
func (cctx *TunnelConversionContext) ToConnectionDetails() managed.ConnectionDetails {
	cd := managed.ConnectionDetails{}
	if cctx.Domain != "" {
		cd["domain"] = []byte(cctx.Domain)
	}
	if cctx.TunnelToken != "" {
		cd["tunnelToken"] = []byte(cctx.TunnelToken)
	}
	if cctx.TokenID != "" {
		cd["tokenId"] = []byte(cctx.TokenID)
	}
	return cd
}

// ToAnthropicNew converts ForProvider to SDK create params.
//
// A Tunnel has no update endpoint, so there is no ToAnthropicUpdate counterpart:
// the resource is immutable after creation.
func (r *Tunnel) ToAnthropicNew() anthropic.BetaTunnelNewParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaTunnelNewParams{}
	if p.DisplayName != nil {
		params.DisplayName = anthropic.String(*p.DisplayName)
	}
	return params
}

// FromAnthropicObservation populates AtProvider from the SDK Get response.
// ArchivedAt is intentionally omitted: its zero value is ambiguous and an
// archived tunnel is treated as deleted by the reconciler. The connector token
// is never stored in AtProvider — it is published as a connection detail.
func (r *Tunnel) FromAnthropicObservation(resp anthropic.BetaTunnel) {
	ap := &r.Status.AtProvider

	ap.ID = &resp.ID
	ap.DisplayName = &resp.DisplayName
	ap.Domain = &resp.Domain

	createdAt := resp.CreatedAt.Format(time.RFC3339)
	ap.CreatedAt = &createdAt
}

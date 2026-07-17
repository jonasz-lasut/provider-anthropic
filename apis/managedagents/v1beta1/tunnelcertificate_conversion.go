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

// TunnelCertificateConversionContext carries the resolved PEM-encoded CA
// certificate (read from CaCertificatePemSecretRef) so ToAnthropicNew can build
// the SDK create params without touching Kubernetes.
//
// There is no ToConnectionDetails: a CA certificate is public input material,
// not a credential to publish. The certificate is create-only, so the value is
// only resolved in the reconciler's Create.
type TunnelCertificateConversionContext struct {
	// CaCertificatePem is the resolved PEM certificate; empty if not set.
	CaCertificatePem string
}

// ToAnthropicNew converts ForProvider to SDK create params. The parent tunnel
// ID is threaded separately as a positional argument by the reconciler, so it
// is not set here.
//
// A TunnelCertificate has no update endpoint, so there is no ToAnthropicUpdate
// counterpart: the resource is immutable after creation.
func (r *TunnelCertificate) ToAnthropicNew(ctx *TunnelCertificateConversionContext) anthropic.BetaTunnelCertificateNewParams {
	params := anthropic.BetaTunnelCertificateNewParams{}
	if ctx != nil && ctx.CaCertificatePem != "" {
		params.CaCertificatePem = ctx.CaCertificatePem
	}
	return params
}

// FromAnthropicObservation populates AtProvider from the SDK Get response.
// ArchivedAt is intentionally omitted: its zero value is ambiguous and an
// archived certificate is treated as deleted by the reconciler.
func (r *TunnelCertificate) FromAnthropicObservation(resp anthropic.BetaTunnelCertificate) {
	ap := &r.Status.AtProvider

	ap.ID = &resp.ID
	ap.TunnelID = &resp.TunnelID
	ap.Fingerprint = &resp.Fingerprint

	expiresAt := resp.ExpiresAt.Format(time.RFC3339)
	ap.ExpiresAt = &expiresAt
	createdAt := resp.CreatedAt.Format(time.RFC3339)
	ap.CreatedAt = &createdAt
}

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

// MemoryStoreMemoryConversionContext carries reconcile-time values for
// MemoryStoreMemory SDK param construction.
type MemoryStoreMemoryConversionContext struct {
	Content string // resolved from ContentSecretRef; empty if not set
}

func (r *MemoryStoreMemory) ToAnthropicNew(ctx *MemoryStoreMemoryConversionContext) anthropic.BetaMemoryStoreMemoryNewParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaMemoryStoreMemoryNewParams{}
	if p.Path != nil {
		params.Path = *p.Path
	}
	if ctx != nil && ctx.Content != "" {
		params.Content = anthropic.String(ctx.Content)
	}
	return params
}

func (r *MemoryStoreMemory) ToAnthropicUpdate(ctx *MemoryStoreMemoryConversionContext) anthropic.BetaMemoryStoreMemoryUpdateParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaMemoryStoreMemoryUpdateParams{}
	if p.MemoryStoreID != nil {
		params.MemoryStoreID = *p.MemoryStoreID
	}
	if p.Path != nil {
		params.Path = anthropic.String(*p.Path)
	}
	if ctx != nil && ctx.Content != "" {
		params.Content = anthropic.String(ctx.Content)
	}
	return params
}

func (r *MemoryStoreMemory) FromAnthropicObservation(resp anthropic.BetaManagedAgentsMemory) {
	r.Status.AtProvider.ID = &resp.ID
	r.Status.AtProvider.MemoryStoreID = &resp.MemoryStoreID
	r.Status.AtProvider.MemoryVersionID = &resp.MemoryVersionID
	r.Status.AtProvider.ContentSha256 = &resp.ContentSha256
	r.Status.AtProvider.ContentSizeBytes = &resp.ContentSizeBytes
	r.Status.AtProvider.Path = &resp.Path
	createdAt := resp.CreatedAt.Format(time.RFC3339)
	r.Status.AtProvider.CreatedAt = &createdAt
	updatedAt := resp.UpdatedAt.Format(time.RFC3339)
	r.Status.AtProvider.UpdatedAt = &updatedAt
	// Content intentionally omitted — not stored in AtProvider
}

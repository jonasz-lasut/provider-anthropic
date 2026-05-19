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

func (r *MemoryStore) ToAnthropicNew() anthropic.BetaMemoryStoreNewParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaMemoryStoreNewParams{}
	if p.Name != nil {
		params.Name = *p.Name
	}
	if p.Description != nil {
		params.Description = anthropic.String(*p.Description)
	}
	if p.Metadata != nil {
		params.Metadata = p.Metadata
	}
	return params
}

func (r *MemoryStore) ToAnthropicUpdate() anthropic.BetaMemoryStoreUpdateParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaMemoryStoreUpdateParams{}
	if p.Name != nil {
		params.Name = anthropic.String(*p.Name)
	}
	if p.Description != nil {
		params.Description = anthropic.String(*p.Description)
	}
	if p.Metadata != nil {
		params.Metadata = p.Metadata
	}
	return params
}

func (r *MemoryStore) FromAnthropicObservation(resp anthropic.BetaManagedAgentsMemoryStore) {
	r.Status.AtProvider.ID = &resp.ID
	r.Status.AtProvider.Name = &resp.Name
	r.Status.AtProvider.Description = &resp.Description
	r.Status.AtProvider.Metadata = resp.Metadata
	createdAt := resp.CreatedAt.Format(time.RFC3339)
	r.Status.AtProvider.CreatedAt = &createdAt
	updatedAt := resp.UpdatedAt.Format(time.RFC3339)
	r.Status.AtProvider.UpdatedAt = &updatedAt
	// ArchivedAt intentionally omitted
}

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

// ToAnthropicNew converts ForProvider to SDK create params.
//
// A Dream has no update endpoint, so there is no ToAnthropicUpdate counterpart:
// the resource is immutable after creation.
func (r *Dream) ToAnthropicNew() anthropic.BetaDreamNewParams {
	p := r.Spec.ForProvider
	params := anthropic.BetaDreamNewParams{}

	for _, in := range p.Inputs {
		params.Inputs = append(params.Inputs, dreamInputToParam(in))
	}

	if p.Model != nil {
		mc := anthropic.BetaDreamModelConfigParam{}
		if p.Model.ID != nil {
			mc.ID = *p.Model.ID
		}
		if p.Model.Speed != nil {
			mc.Speed = anthropic.BetaDreamModelConfigParamSpeed(*p.Model.Speed)
		}
		params.Model = anthropic.BetaDreamNewParamsModelUnion{OfBetaDreamModelConfig: &mc}
	}

	if p.Instructions != nil {
		params.Instructions = anthropic.String(*p.Instructions)
	}

	return params
}

func dreamInputToParam(in DreamInput) anthropic.BetaDreamInputUnionParam {
	inType := ""
	if in.Type != nil {
		inType = *in.Type
	}
	switch inType {
	case "memory_store":
		ms := anthropic.BetaDreamMemoryStoreInputParam{
			Type: anthropic.BetaDreamMemoryStoreInputTypeMemoryStore,
		}
		if in.MemoryStoreID != nil {
			ms.MemoryStoreID = *in.MemoryStoreID
		}
		return anthropic.BetaDreamInputUnionParam{OfMemoryStore: &ms}
	case "sessions":
		ses := anthropic.BetaDreamSessionsInputParam{
			Type:       anthropic.BetaDreamSessionsInputTypeSessions,
			SessionIDs: in.SessionIDs,
		}
		return anthropic.BetaDreamInputUnionParam{OfSessions: &ses}
	}
	return anthropic.BetaDreamInputUnionParam{}
}

// FromAnthropicObservation populates AtProvider from the SDK Get response.
// ArchivedAt is intentionally omitted: its zero value is ambiguous and an
// archived dream is treated as deleted by the reconciler.
func (r *Dream) FromAnthropicObservation(resp anthropic.BetaDream) {
	ap := &r.Status.AtProvider

	ap.ID = &resp.ID
	status := string(resp.Status)
	ap.Status = &status
	ap.Instructions = &resp.Instructions
	ap.SessionID = &resp.SessionID

	model := &DreamModelObservation{ID: &resp.Model.ID}
	if resp.Model.Speed != "" {
		speed := string(resp.Model.Speed)
		model.Speed = &speed
	}
	ap.Model = model

	ap.Inputs = nil
	for _, in := range resp.Inputs {
		obs := DreamInputObservation{}
		if in.Type != "" {
			t := in.Type
			obs.Type = &t
		}
		switch in.Type {
		case "memory_store":
			id := in.MemoryStoreID
			obs.MemoryStoreID = &id
		case "sessions":
			obs.SessionIDs = in.SessionIDs
		}
		ap.Inputs = append(ap.Inputs, obs)
	}

	ap.Outputs = nil
	for _, out := range resp.Outputs {
		obs := DreamOutputObservation{MemoryStoreID: &out.MemoryStoreID}
		if out.Type != "" {
			t := string(out.Type)
			obs.Type = &t
		}
		ap.Outputs = append(ap.Outputs, obs)
	}

	if resp.Error.Message != "" || resp.Error.Type != "" {
		ap.Error = &DreamErrorObservation{
			Message: &resp.Error.Message,
			Type:    &resp.Error.Type,
		}
	} else {
		ap.Error = nil
	}

	ap.Usage = &DreamUsageObservation{
		CacheCreationInputTokens: &resp.Usage.CacheCreationInputTokens,
		CacheReadInputTokens:     &resp.Usage.CacheReadInputTokens,
		InputTokens:              &resp.Usage.InputTokens,
		OutputTokens:             &resp.Usage.OutputTokens,
	}

	createdAt := resp.CreatedAt.Format(time.RFC3339)
	ap.CreatedAt = &createdAt
	if !resp.EndedAt.IsZero() {
		endedAt := resp.EndedAt.Format(time.RFC3339)
		ap.EndedAt = &endedAt
	} else {
		ap.EndedAt = nil
	}
}

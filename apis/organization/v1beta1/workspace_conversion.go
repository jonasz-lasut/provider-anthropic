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
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
)

// AllowedInferenceGeosUnrestricted is the single AllowedInferenceGeos entry
// that maps to the API's "unrestricted" union variant.
const AllowedInferenceGeosUnrestricted = "unrestricted"

// ToAnthropicNew converts ForProvider to OrganizationWorkspaceNewParams.
func (r *Workspace) ToAnthropicNew() anthropic.OrganizationWorkspaceNewParams {
	p := r.Spec.ForProvider
	params := anthropic.OrganizationWorkspaceNewParams{}
	if p.Name != nil {
		params.Name = *p.Name
	}
	if p.DisplayColor != nil {
		params.DisplayColor = anthropic.String(*p.DisplayColor)
	}
	if p.ExternalKeyID != nil {
		params.ExternalKeyID = anthropic.String(*p.ExternalKeyID)
	}
	if p.Tags != nil {
		params.Tags = p.Tags
	}
	if dr := p.DataResidency; dr != nil {
		if dr.WorkspaceGeo != nil {
			params.DataResidency.WorkspaceGeo = anthropic.DataResidencyCreateConfigWorkspaceGeo(*dr.WorkspaceGeo)
		}
		if dr.DefaultInferenceGeo != nil {
			params.DataResidency.DefaultInferenceGeo = anthropic.DataResidencyCreateConfigDefaultInferenceGeo(*dr.DefaultInferenceGeo)
		}
		if dr.AllowedInferenceGeos != nil {
			if isUnrestricted(dr.AllowedInferenceGeos) {
				params.DataResidency.AllowedInferenceGeos.OfUnrestricted = constant.ValueOf[constant.Unrestricted]()
			} else {
				params.DataResidency.AllowedInferenceGeos.OfGeos = allowedGeos(dr.AllowedInferenceGeos)
			}
		}
	}
	return params
}

// ToAnthropicUpdate converts ForProvider to OrganizationWorkspaceUpdateParams.
// WorkspaceGeo is immutable and never sent. ExternalKeyID is write-once, so it
// is only sent when it differs from the observed value.
func (r *Workspace) ToAnthropicUpdate() anthropic.OrganizationWorkspaceUpdateParams {
	p := r.Spec.ForProvider
	params := anthropic.OrganizationWorkspaceUpdateParams{}
	if p.Name != nil {
		params.Name = anthropic.String(*p.Name)
	}
	if p.DisplayColor != nil {
		params.DisplayColor = anthropic.String(*p.DisplayColor)
	}
	if p.ExternalKeyID != nil {
		observed := r.Status.AtProvider.ExternalKeyID
		if observed == nil || *observed != *p.ExternalKeyID {
			params.ExternalKeyID = anthropic.String(*p.ExternalKeyID)
		}
	}
	if p.Tags != nil {
		params.Tags = p.Tags
	}
	if dr := p.DataResidency; dr != nil {
		if dr.DefaultInferenceGeo != nil {
			params.DataResidency.DefaultInferenceGeo = anthropic.DataResidencyUpdateConfigDefaultInferenceGeo(*dr.DefaultInferenceGeo)
		}
		if dr.AllowedInferenceGeos != nil {
			if isUnrestricted(dr.AllowedInferenceGeos) {
				params.DataResidency.AllowedInferenceGeos.OfUnrestricted = constant.ValueOf[constant.Unrestricted]()
			} else {
				params.DataResidency.AllowedInferenceGeos.OfGeos = allowedGeos(dr.AllowedInferenceGeos)
			}
		}
	}
	return params
}

// FromAnthropicObservation populates AtProvider from a Workspace.
// ArchivedAt is intentionally omitted: the reconciler treats an archived
// workspace as absent.
func (r *Workspace) FromAnthropicObservation(resp anthropic.Workspace) {
	r.Status.AtProvider.ID = &resp.ID
	r.Status.AtProvider.Name = &resp.Name
	r.Status.AtProvider.DisplayColor = &resp.DisplayColor
	r.Status.AtProvider.ExternalKeyID = nil
	if resp.ExternalKeyID != "" {
		r.Status.AtProvider.ExternalKeyID = &resp.ExternalKeyID
	}
	r.Status.AtProvider.Tags = resp.Tags
	r.Status.AtProvider.DataResidency = dataResidencyObservation(resp.DataResidency)
	r.Status.AtProvider.CompartmentID = &resp.CompartmentID
	createdAt := resp.CreatedAt.Format(time.RFC3339)
	r.Status.AtProvider.CreatedAt = &createdAt
}

func dataResidencyObservation(dr anthropic.DataResidency) *WorkspaceDataResidency {
	out := &WorkspaceDataResidency{}
	if dr.WorkspaceGeo != "" {
		out.WorkspaceGeo = new(string(dr.WorkspaceGeo))
	}
	if dr.DefaultInferenceGeo != "" {
		out.DefaultInferenceGeo = new(string(dr.DefaultInferenceGeo))
	}
	switch {
	case dr.AllowedInferenceGeos.OfUnrestricted != "":
		out.AllowedInferenceGeos = []string{AllowedInferenceGeosUnrestricted}
	case len(dr.AllowedInferenceGeos.OfGeos) > 0:
		out.AllowedInferenceGeos = observedGeos(dr.AllowedInferenceGeos.OfGeos)
	}
	return out
}

func observedGeos(geos []anthropic.AllowedInferenceGeo) []string {
	out := make([]string, 0, len(geos))
	for _, g := range geos {
		out = append(out, string(g))
	}
	return out
}

func isUnrestricted(geos []string) bool {
	return len(geos) == 1 && geos[0] == AllowedInferenceGeosUnrestricted
}

func allowedGeos(geos []string) []anthropic.AllowedInferenceGeo {
	out := make([]anthropic.AllowedInferenceGeo, 0, len(geos))
	for _, g := range geos {
		out = append(out, anthropic.AllowedInferenceGeo(g))
	}
	return out
}

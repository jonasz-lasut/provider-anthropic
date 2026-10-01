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

package agent

import (
	"encoding/json"
	"testing"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/google/go-cmp/cmp"

	v1beta1 "github.com/jonasz-lasut/provider-anthropic/apis/managedagents/v1beta1"
)

// toolsResponse is shaped like a live Managed Agents response to a toolset
// sent with default_config {permission_policy: always_allow} and overrides
// for bash and web_fetch: only the overrides come back, resolved and in order.
const toolsResponse = `{
  "id": "agent_1",
  "tools": [
    {
      "type": "agent_toolset_20260401",
      "default_config": {"enabled": true, "permission_policy": {"type": "always_allow"}},
      "configs": [
        {"name": "bash", "type": "bash", "enabled": true, "permission_policy": {"type": "auto"}},
        {"name": "web_fetch", "type": "web_fetch", "enabled": false, "permission_policy": {"type": "always_allow"}}
      ]
    }
  ]
}`

func TestIsUpToDateToolConfigs(t *testing.T) {
	cases := map[string]struct {
		args []v1beta1.AgentToolConfig
		want bool
	}{
		"OverridesMatchResolvedResponse": {
			args: []v1beta1.AgentToolConfig{{
				Type:          new("agent_toolset_20260401"),
				DefaultConfig: &v1beta1.AgentToolsetDefaultConfig{PermissionPolicy: new("always_allow")},
				Configs: []v1beta1.AgentToolOverride{
					{Name: new("bash"), PermissionPolicy: new("auto")},
					{Name: new("web_fetch"), Enabled: new(false)},
				},
			}},
			want: true,
		},
		"OmittedConfigsAreNotDrift": {
			args: []v1beta1.AgentToolConfig{{Type: new("agent_toolset_20260401")}},
			want: true,
		},
		"ChangedPolicyIsDrift": {
			args: []v1beta1.AgentToolConfig{{
				Type: new("agent_toolset_20260401"),
				Configs: []v1beta1.AgentToolOverride{
					{Name: new("bash"), PermissionPolicy: new("always_ask")},
					{Name: new("web_fetch"), Enabled: new(false)},
				},
			}},
			want: false,
		},
		"AddedOverrideIsDrift": {
			args: []v1beta1.AgentToolConfig{{
				Type: new("agent_toolset_20260401"),
				Configs: []v1beta1.AgentToolOverride{
					{Name: new("bash"), PermissionPolicy: new("auto")},
					{Name: new("web_fetch"), Enabled: new(false)},
					{Name: new("grep"), Enabled: new(false)},
				},
			}},
			want: false,
		},
	}

	var resp anthropic.BetaManagedAgentsAgent
	if err := json.Unmarshal([]byte(toolsResponse), &resp); err != nil {
		t.Fatalf("json.Unmarshal(): %v", err)
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ag := &v1beta1.Agent{Spec: v1beta1.AgentSpec{ForProvider: v1beta1.AgentParameters{Tools: tc.args}}}
			ag.FromAnthropicObservation(resp)

			got := isUpToDate(ag, "")

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("isUpToDate(): -want, +got:\n%s", diff)
			}
		})
	}
}

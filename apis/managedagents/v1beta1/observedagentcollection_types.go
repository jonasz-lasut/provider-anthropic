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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpcommon "github.com/crossplane/crossplane-runtime/v2/apis/common"
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
)

// ObservedAgentCollectionSpec defines the desired state of
// ObservedAgentCollection.
type ObservedAgentCollectionSpec struct {
	// ProviderConfigReference selects the ProviderConfig used to authenticate
	// against the Anthropic API for this collection.
	// +kubebuilder:default={"name": "default", "kind": "ClusterProviderConfig"}
	ProviderConfigReference xpcommon.ProviderConfigReference `json:"providerConfigRef,omitempty"`

	// Predicates filter the Anthropic List(agents) call. The reconciler maps
	// fields to BetaAgentListParams when supported server-side and applies
	// any remaining predicates client-side.
	// +optional
	Predicates *Predicates `json:"predicates,omitempty"`

	// Template is applied to every Agent child created by this collection.
	// +optional
	Template *ObservedCollectionTemplate `json:"template,omitempty"`
}

// ObservedCollectionTemplate carries metadata applied to children created by
// any Observed<R>Collection. Defined here because ObservedAgentCollection is
// the first collection kind; subsequent kinds reference this type.
type ObservedCollectionTemplate struct {
	Metadata ObservedCollectionTemplateMetadata `json:"metadata,omitempty"`
}

// ObservedCollectionTemplateMetadata is the label/annotation overlay applied
// to children.
type ObservedCollectionTemplateMetadata struct {
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ObservedCollectionMember is one matched child managed resource.
type ObservedCollectionMember struct {
	// Name of the child resource in the same namespace.
	Name string `json:"name"`

	// ID is the Anthropic-assigned identifier of the observed resource.
	ID string `json:"id"`
}

// ObservedAgentCollectionStatus defines the observed state of
// ObservedAgentCollection.
type ObservedAgentCollectionStatus struct {
	xpv1.ResourceStatus `json:",inline"`

	// MembershipLabel is the label applied to every child Agent owned by
	// this collection. Useful for `kubectl get agents -l <key>=<value>` queries.
	// +optional
	MembershipLabel map[string]string `json:"membershipLabel,omitempty"`

	// Items references children currently materialized by this collection.
	// +optional
	Items []ObservedCollectionMember `json:"items,omitempty"`

	// ItemCount mirrors len(Items) for use in a printer column (JSONPath
	// does not support array length).
	// +optional
	ItemCount *int32 `json:"itemCount,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="ITEMS",type="integer",JSONPath=".status.itemCount"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,anthropic},shortName=obscoll-agent
//
// ObservedAgentCollection materializes one Observe-only Agent per remote
// agent matched by spec.predicates.
type ObservedAgentCollection struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ObservedAgentCollectionSpec   `json:"spec"`
	Status ObservedAgentCollectionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
//
// ObservedAgentCollectionList contains a list of ObservedAgentCollection.
type ObservedAgentCollectionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ObservedAgentCollection `json:"items"`
}

var (
	ObservedAgentCollectionKind             = "ObservedAgentCollection"
	ObservedAgentCollectionGroupVersionKind = GroupVersion.WithKind(ObservedAgentCollectionKind)
)

func init() {
	SchemeBuilder.Register(&ObservedAgentCollection{}, &ObservedAgentCollectionList{})
}

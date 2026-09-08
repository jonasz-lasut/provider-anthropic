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
	"k8s.io/apimachinery/pkg/runtime"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// ServiceAccountParameters defines the desired state of an Anthropic
// ServiceAccount. These fields map to BetaOrganizationServiceAccountNewParams /
// BetaOrganizationServiceAccountUpdateParams from the Anthropic SDK.
type ServiceAccountParameters struct {
	// Required: Name of the service account. Immutable after creation.
	// +optional
	Name *string `json:"name,omitempty"`

	// Description is free text.
	// +optional
	Description *string `json:"description,omitempty"`

	// OrganizationRole is developer (the API default) or admin. Creating an
	// admin-role service account requires an interactive credential; a
	// federation rule may only grant org:admin scope to an admin-role target.
	// +optional
	// +kubebuilder:validation:Enum=developer;admin
	OrganizationRole *string `json:"organizationRole,omitempty"`
}

// ServiceAccountObservation holds the observed state of an Anthropic
// ServiceAccount as returned by the API. These fields are read-only.
type ServiceAccountObservation struct {
	// ID is the Anthropic-assigned service account identifier (svac_...).
	// Also stored in the external-name annotation, which the reconciler uses
	// as the primary key.
	// +optional
	ID *string `json:"id,omitempty"`

	// Name is the observed name.
	// +optional
	Name *string `json:"name,omitempty"`

	// Description is the observed description.
	// +optional
	Description *string `json:"description,omitempty"`

	// OrganizationRole is the observed organization role.
	// +optional
	OrganizationRole *string `json:"organizationRole,omitempty"`

	// CreatedAt is the RFC 3339 timestamp when the service account was created.
	// +optional
	CreatedAt *string `json:"createdAt,omitempty"`

	// UpdatedAt is the RFC 3339 timestamp of the last modification.
	// +optional
	UpdatedAt *string `json:"updatedAt,omitempty"`

	// CreatedByActorID identifies who created the service account.
	// +optional
	CreatedByActorID *string `json:"createdByActorId,omitempty"`

	// UpdatedByActorID identifies who last modified the service account.
	// +optional
	UpdatedByActorID *string `json:"updatedByActorId,omitempty"`

	// ArchivedAt is set when the service account has been archived.
	// +optional
	ArchivedAt *string `json:"archivedAt,omitempty"`
}

// ServiceAccountSpec defines the desired state of ServiceAccount.
type ServiceAccountSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`

	// ForProvider holds the configuration the provider reconciles against the
	// Anthropic API on every loop.
	ForProvider ServiceAccountParameters `json:"forProvider"`
}

// ServiceAccountStatus defines the observed state of ServiceAccount.
type ServiceAccountStatus struct {
	xpv2.ManagedResourceStatus `json:",inline"`

	// AtProvider holds the observed state as returned by the Anthropic API.
	// +optional
	AtProvider ServiceAccountObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,anthropic},shortName=svac
//
// ServiceAccount is a managed resource representing a workload identity in an
// Anthropic organization (Admin API), the target of federation rules. The
// endpoint accepts only org:admin OAuth tokens, so it requires a
// ProviderConfig with the WorkloadIdentityFederation identity.
type ServiceAccount struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceAccountSpec   `json:"spec"`
	Status ServiceAccountStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
//
// ServiceAccountList contains a list of ServiceAccount.
type ServiceAccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceAccount `json:"items"`
}

// ServiceAccountKind and ServiceAccountGroupVersionKind are used by
// controller setup.
var (
	ServiceAccountKind             = "ServiceAccount"
	ServiceAccountGroupVersionKind = GroupVersion.WithKind(ServiceAccountKind)
)

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &ServiceAccount{}, &ServiceAccountList{})
		return nil
	})
}

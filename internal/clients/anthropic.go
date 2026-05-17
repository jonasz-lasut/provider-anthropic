/*
Copyright 2025 The provider-anthropic-platform Authors.

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

// Package clients provides a thin wrapper around the Anthropic SDK client.
package clients

import (
	"context"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	xpcommon "github.com/crossplane/crossplane-runtime/v2/apis/common"
	xperrors "github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	xpresource "github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pcv1alpha1 "github.com/jonasz-lasut/provider-anthropic-platform/apis/config/v1alpha1"
)

const (
	errNoProviderConfig  = "no providerConfigRef provided"
	errGetProviderConfig = "cannot get ProviderConfig"
	errTrackUsage        = "cannot track ProviderConfig usage"
	errGetCredentials    = "cannot get credentials"
)

// NewClient returns an Anthropic SDK client authenticated with the credentials
// referenced by the supplied managed resource's ProviderConfig. It also tracks
// usage via ProviderConfigUsage.
func NewClient(ctx context.Context, crClient client.Client, mg xpresource.ModernManaged) (*anthropic.Client, error) {
	pcSpec, err := resolveProviderConfig(ctx, crClient, mg.GetProviderConfigReference(), mg.GetNamespace())
	if err != nil {
		return nil, err
	}

	t := xpresource.NewProviderConfigUsageTracker(crClient, &pcv1alpha1.ProviderConfigUsage{})
	if err := t.Track(ctx, mg); err != nil {
		return nil, xperrors.Wrap(err, errTrackUsage)
	}

	return buildClientFromSpec(ctx, crClient, pcSpec)
}

// NewClientFromProviderConfig returns an Anthropic SDK client authenticated
// with the credentials referenced by the supplied ProviderConfig reference.
// It is intended for callers that are not Crossplane managed resources (such
// as the Observed<R>Collection reconcilers) and therefore cannot use the
// ProviderConfigUsage tracker.
func NewClientFromProviderConfig(
	ctx context.Context,
	crClient client.Client,
	configRef *xpcommon.ProviderConfigReference,
	namespace string,
) (*anthropic.Client, error) {
	pcSpec, err := resolveProviderConfig(ctx, crClient, configRef, namespace)
	if err != nil {
		return nil, err
	}
	return buildClientFromSpec(ctx, crClient, pcSpec)
}

// buildClientFromSpec extracts credentials from the resolved ProviderConfig
// spec and constructs an authenticated Anthropic SDK client.
func buildClientFromSpec(ctx context.Context, crClient client.Client, pcSpec *pcv1alpha1.ProviderConfigSpec) (*anthropic.Client, error) {
	creds, err := xpresource.CommonCredentialExtractor(
		ctx,
		pcSpec.Credentials.Source,
		crClient,
		pcSpec.Credentials.CommonCredentialSelectors,
	)
	if err != nil {
		return nil, xperrors.Wrap(err, errGetCredentials)
	}

	// Only API-key authentication is supported today.
	c := anthropic.NewClient(option.WithAPIKey(string(creds)))
	return &c, nil
}

func resolveProviderConfig(
	ctx context.Context,
	crClient client.Client,
	configRef *xpcommon.ProviderConfigReference,
	namespace string,
) (*pcv1alpha1.ProviderConfigSpec, error) {
	if configRef == nil {
		return nil, xperrors.New(errNoProviderConfig)
	}

	pcRuntimeObj, err := crClient.Scheme().New(pcv1alpha1.SchemeGroupVersion.WithKind(configRef.Kind))
	if err != nil {
		return nil, xperrors.Wrapf(err, "referenced provider config kind %q is invalid", configRef.Kind)
	}
	pcObj, ok := pcRuntimeObj.(xpresource.ProviderConfig)
	if !ok {
		return nil, xperrors.Errorf("referenced provider config kind %q is not a provider config type", configRef.Kind)
	}

	// Namespace is ignored if the PC is a cluster-scoped type.
	if err := crClient.Get(ctx, types.NamespacedName{Name: configRef.Name, Namespace: namespace}, pcObj); err != nil {
		return nil, xperrors.Wrap(err, errGetProviderConfig)
	}

	switch pc := pcObj.(type) {
	case *pcv1alpha1.ProviderConfig:
		// Patch the in-memory PC so local SecretRefs resolve in the caller's
		// namespace. The fetched object is not cached, so in-place mutation is safe.
		if pc.Spec.Credentials.SecretRef != nil {
			pc.Spec.Credentials.SecretRef.Namespace = namespace
		}
		return &pc.Spec, nil
	case *pcv1alpha1.ClusterProviderConfig:
		return &pc.Spec, nil
	default:
		return nil, xperrors.New("unknown ProviderConfig type")
	}
}

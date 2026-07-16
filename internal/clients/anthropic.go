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

// Package clients provides a thin wrapper around the Anthropic SDK client.
package clients

import (
	"context"
	"encoding/json"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	xperrors "github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	xpresource "github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pcv1beta1 "github.com/jonasz-lasut/provider-anthropic/apis/config/v1beta1"
)

const (
	errNoProviderConfig     = "no providerConfigRef provided"
	errGetProviderConfig    = "cannot get ProviderConfig"
	errTrackUsage           = "cannot track ProviderConfig usage"
	errGetCredentials       = "cannot get credentials"
	errNoIdentity           = "spec.identity is required but not set"
	errUnmarshalCredentials = "cannot unmarshal Anthropic credentials as JSON"
	errMissingAPIKey        = "identity type is APIKey but credentials JSON has no \"api_key\" field"
	errUnknownIdentityType  = "unknown identity type %q"
)

// NewClient returns an Anthropic SDK client authenticated with the credentials
// referenced by the supplied managed resource's ProviderConfig. It also tracks
// usage via ProviderConfigUsage.
func NewClient(ctx context.Context, crClient client.Client, mg xpresource.ModernManaged) (*anthropic.Client, error) {
	pcSpec, err := resolveProviderConfig(ctx, crClient, mg.GetProviderConfigReference(), mg.GetNamespace())
	if err != nil {
		return nil, err
	}

	t := xpresource.NewProviderConfigUsageTracker(crClient, &pcv1beta1.ProviderConfigUsage{})
	if err := t.Track(ctx, mg); err != nil {
		return nil, xperrors.Wrap(err, errTrackUsage)
	}

	return buildClientFromSpec(ctx, crClient, pcSpec)
}

// buildClientFromSpec extracts credentials from the resolved ProviderConfig
// spec and constructs an authenticated Anthropic SDK client.
func buildClientFromSpec(ctx context.Context, crClient client.Client, pcSpec *pcv1beta1.ProviderConfigSpec) (*anthropic.Client, error) {
	if pcSpec.Identity == nil {
		return nil, xperrors.New(errNoIdentity)
	}

	creds, err := xpresource.CommonCredentialExtractor(
		ctx,
		pcSpec.Credentials.Source,
		crClient,
		pcSpec.Credentials.CommonCredentialSelectors,
	)
	if err != nil {
		return nil, xperrors.Wrap(err, errGetCredentials)
	}

	apiKey, err := apiKeyFromCredentials(creds, pcSpec.Identity.Type)
	if err != nil {
		return nil, err
	}

	c := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &c, nil
}

// apiKeyFromCredentials parses the JSON credentials payload and extracts the
// Anthropic API key according to the configured identity type. The payload is
// a JSON object, e.g. {"api_key": "sk-ant-..."}.
func apiKeyFromCredentials(data []byte, identityType pcv1beta1.IdentityType) (string, error) {
	creds := map[string]string{}
	if err := json.Unmarshal(data, &creds); err != nil {
		return "", xperrors.Wrap(err, errUnmarshalCredentials)
	}

	switch identityType {
	case pcv1beta1.IdentityTypeAPIKey:
		apiKey := creds["api_key"]
		if apiKey == "" {
			return "", xperrors.New(errMissingAPIKey)
		}
		return apiKey, nil
	default:
		return "", xperrors.Errorf(errUnknownIdentityType, identityType)
	}
}

func resolveProviderConfig(
	ctx context.Context,
	crClient client.Client,
	configRef *xpv2.ProviderConfigReference,
	namespace string,
) (*pcv1beta1.ProviderConfigSpec, error) {
	if configRef == nil {
		return nil, xperrors.New(errNoProviderConfig)
	}

	pcRuntimeObj, err := crClient.Scheme().New(pcv1beta1.SchemeGroupVersion.WithKind(configRef.Kind))
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
	case *pcv1beta1.ProviderConfig:
		// Patch the in-memory PC so local SecretRefs resolve in the caller's
		// namespace. The fetched object is not cached, so in-place mutation is safe.
		if pc.Spec.Credentials.SecretRef != nil {
			pc.Spec.Credentials.SecretRef.Namespace = namespace
		}
		return &pc.Spec, nil
	case *pcv1beta1.ClusterProviderConfig:
		return &pc.Spec, nil
	default:
		return nil, xperrors.New("unknown ProviderConfig type")
	}
}

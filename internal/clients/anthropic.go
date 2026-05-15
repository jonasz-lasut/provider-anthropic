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

// NewClient returns a pointer to an Anthropic SDK client authenticated with apiKey.
func NewClient(ctx context.Context, crClient client.Client, mg xpresource.ModernManaged) (*anthropic.Client, error) {
	pcSpec, err := resolveProviderConfigModern(ctx, crClient, mg)
	if err != nil {
		return nil, err
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

	// only handle api key for now
	c := anthropic.NewClient(option.WithAPIKey(string(creds)))
	return &c, nil
}

func resolveProviderConfigModern(ctx context.Context, crClient client.Client, mg xpresource.ModernManaged) (*pcv1alpha1.ProviderConfigSpec, error) {
	configRef := mg.GetProviderConfigReference()
	if configRef == nil {
		return nil, xperrors.New(errNoProviderConfig)
	}

	pcRuntimeObj, err := crClient.Scheme().New(pcv1alpha1.SchemeGroupVersion.WithKind(configRef.Kind))
	if err != nil {
		return nil, xperrors.Wrapf(err, "referenced provider config kind %q is invalid for %s/%s", configRef.Kind, mg.GetNamespace(), mg.GetName())
	}
	pcObj, ok := pcRuntimeObj.(xpresource.ProviderConfig)
	if !ok {
		return nil, xperrors.Errorf("referenced provider config kind %q is not a provider config type %s/%s", configRef.Kind, mg.GetNamespace(), mg.GetName())
	}

	// Namespace will be ignored if the PC is a cluster-scoped type
	if err := crClient.Get(ctx, types.NamespacedName{Name: configRef.Name, Namespace: mg.GetNamespace()}, pcObj); err != nil {
		return nil, xperrors.Wrap(err, errGetProviderConfig)
	}

	var pcSpec pcv1alpha1.ProviderConfigSpec
	switch pc := pcObj.(type) {
	case *pcv1alpha1.ProviderConfig:
		enrichLocalSecretRefs(pc, mg)
		pcSpec = pc.Spec
	case *pcv1alpha1.ClusterProviderConfig:
		pcSpec = pc.Spec
	default:
		return nil, xperrors.New("unknown")
	}
	t := xpresource.NewProviderConfigUsageTracker(crClient, &pcv1alpha1.ProviderConfigUsage{})
	if err := t.Track(ctx, mg); err != nil {
		return nil, xperrors.Wrap(err, errTrackUsage)
	}
	return &pcSpec, nil
}

func enrichLocalSecretRefs(pc *pcv1alpha1.ProviderConfig, mg xpresource.Managed) {
	if pc != nil && pc.Spec.Credentials.SecretRef != nil {
		pc.Spec.Credentials.SecretRef.Namespace = mg.GetNamespace()
	}
}

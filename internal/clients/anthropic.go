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
	"os"
	"strings"
	"sync"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	xperrors "github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	xpresource "github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
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
	errNoFederation         = "identity type is WorkloadIdentityFederation but spec.identity.federation is not set"
	errReadIdentityToken    = "cannot read identity token file %q"

	// workspaceIDHeader scopes a request to one Anthropic workspace. Only
	// multi-workspace API keys honour it.
	workspaceIDHeader = "anthropic-workspace-id"

	// defaultFederationTokenFile is where a DeploymentRuntimeConfig is expected
	// to mount the projected ServiceAccount token for federation.
	defaultFederationTokenFile = "/var/run/secrets/anthropic/token"
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

	if pcSpec.Identity.Type == pcv1beta1.IdentityTypeWorkloadIdentityFederation {
		if pcSpec.Identity.Federation == nil {
			return nil, xperrors.New(errNoFederation)
		}
		return federationClient(pcSpec.Identity.Federation, pcSpec.WorkspaceID), nil
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

	c := anthropic.NewClient(clientOptions(apiKey, pcSpec.WorkspaceID)...)
	return &c, nil
}

// clientOptions returns the SDK request options for a ProviderConfig: the API
// key and, when set, the workspace every request is scoped to. The workspace
// header is applied client-wide, so no per-resource wiring is needed.
func clientOptions(apiKey string, workspaceID *string) []option.RequestOption {
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if workspaceID != nil && *workspaceID != "" {
		opts = append(opts, option.WithHeader(workspaceIDHeader, *workspaceID))
	}
	return opts
}

// federationClients keeps one SDK client per federation identity for the life
// of the process. The SDK caches the minted access token inside the client
// and re-exchanges it only shortly before expiry, while Anthropic accepts a
// projected token (which carries a jti claim) exactly once. A fresh client per
// reconcile would present the same token again and be rejected as a replay
// until the kubelet rotates the file.
var federationClients = struct {
	sync.Mutex
	byKey map[federationKey]*anthropic.Client
}{byKey: map[federationKey]*anthropic.Client{}}

// federationKey identifies everything that shapes a federated client, so a
// changed ProviderConfig gets a client of its own.
type federationKey struct {
	tokenFile        string
	organizationID   string
	federationRuleID string
	serviceAccountID string
	workspaceID      string
}

// federationClient returns the cached SDK client for a federation identity,
// building it on first use.
func federationClient(fed *pcv1beta1.FederationIdentity, workspaceID *string) *anthropic.Client {
	key := federationKey{
		tokenFile:        federationTokenFile(fed),
		organizationID:   fed.OrganizationID,
		federationRuleID: fed.FederationRuleID,
	}
	if fed.ServiceAccountID != nil {
		key.serviceAccountID = *fed.ServiceAccountID
	}
	if workspaceID != nil {
		key.workspaceID = *workspaceID
	}

	federationClients.Lock()
	defer federationClients.Unlock()
	if c, ok := federationClients.byKey[key]; ok {
		return c
	}
	c := anthropic.NewClient(federationOptions(fed, workspaceID)...)
	federationClients.byKey[key] = &c
	return &c
}

// federationTokenFile is where the projected token is read from.
func federationTokenFile(fed *pcv1beta1.FederationIdentity) string {
	if fed.TokenFile != nil && *fed.TokenFile != "" {
		return *fed.TokenFile
	}
	return defaultFederationTokenFile
}

// federationOptions returns the SDK request options for the
// WorkloadIdentityFederation identity: the SDK exchanges the projected token
// read from fed.TokenFile at /v1/oauth/token and refreshes it on its own.
// Federation tokens are workspace-scoped at exchange time, so the
// ProviderConfig's workspaceID goes into the exchange rather than a header.
func federationOptions(fed *pcv1beta1.FederationIdentity, workspaceID *string) []option.RequestOption {
	fo := option.FederationOptions{
		FederationRuleID: fed.FederationRuleID,
		OrganizationID:   fed.OrganizationID,
	}
	if fed.ServiceAccountID != nil {
		fo.ServiceAccountID = *fed.ServiceAccountID
	}
	if workspaceID != nil {
		fo.WorkspaceID = *workspaceID
	}
	return []option.RequestOption{option.WithFederationTokenProvider(fileIdentityToken(federationTokenFile(fed)), fo)}
}

// fileIdentityToken reads the identity token from path on every exchange, so
// a kubelet-rotated projected token is always current.
func fileIdentityToken(path string) option.IdentityTokenFunc {
	return func(_ context.Context) (string, error) {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", xperrors.Wrapf(err, errReadIdentityToken, path)
		}
		return strings.TrimSpace(string(raw)), nil
	}
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

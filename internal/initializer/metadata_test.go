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

package initializer

import (
	"context"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	betav1alpha1 "github.com/jonasz-lasut/provider-anthropic-platform/apis/beta/v1alpha1"
)

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := betav1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("add betav1alpha1 to scheme: %v", err)
	}
	return s
}

func newMemoryStore(name, namespace string, meta map[string]string, pcName, pcKind string) *betav1alpha1.MemoryStore {
	ms := &betav1alpha1.MemoryStore{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: betav1alpha1.MemoryStoreSpec{
			ForProvider: betav1alpha1.MemoryStoreParameters{Metadata: meta},
		},
	}
	ms.SetGroupVersionKind(betav1alpha1.MemoryStoreGroupVersionKind)
	if pcName != "" || pcKind != "" {
		ms.Spec.ProviderConfigReference = &xpv1.ProviderConfigReference{
			Name: pcName,
			Kind: pcKind,
		}
	}
	return ms
}

func TestInitialize_FreshMR_AddsAllDefaults(t *testing.T) {
	ms := newMemoryStore("my-store", "team-a", nil, "pc-1", "ProviderConfig")
	kube := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(ms).Build()

	i := New(kube, "metadata")
	if err := i.Initialize(context.Background(), ms); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	got := ms.Spec.ForProvider.Metadata
	wantKeys := []string{
		"crossplane-kind",
		"crossplane-name",
		"crossplane-namespace",
		"crossplane-providerconfig",
		"crossplane-providerconfig-kind",
	}
	for _, k := range wantKeys {
		if _, ok := got[k]; !ok {
			t.Errorf("missing default key %q in %v", k, got)
		}
	}
	if got["crossplane-kind"] != "memorystore.beta.anthropic.crossplane.io" {
		t.Errorf("crossplane-kind = %q, want %q", got["crossplane-kind"], "memorystore.beta.anthropic.crossplane.io")
	}
	if got["crossplane-name"] != "my-store" {
		t.Errorf("crossplane-name = %q, want %q", got["crossplane-name"], "my-store")
	}
	if got["crossplane-namespace"] != "team-a" {
		t.Errorf("crossplane-namespace = %q, want %q", got["crossplane-namespace"], "team-a")
	}
	if got["crossplane-providerconfig"] != "pc-1" {
		t.Errorf("crossplane-providerconfig = %q, want %q", got["crossplane-providerconfig"], "pc-1")
	}
	if got["crossplane-providerconfig-kind"] != "ProviderConfig" {
		t.Errorf("crossplane-providerconfig-kind = %q, want %q", got["crossplane-providerconfig-kind"], "ProviderConfig")
	}
}

func TestInitialize_PreservesUserKeys(t *testing.T) {
	user := map[string]string{"team": "platform", "env": "prod"}
	ms := newMemoryStore("ms", "ns", user, "pc", "ProviderConfig")
	kube := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(ms).Build()

	if err := New(kube, "metadata").Initialize(context.Background(), ms); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	got := ms.Spec.ForProvider.Metadata
	if got["team"] != "platform" {
		t.Errorf("user key team lost: %v", got)
	}
	if got["env"] != "prod" {
		t.Errorf("user key env lost: %v", got)
	}
	wantKeys := []string{
		"crossplane-kind",
		"crossplane-name",
		"crossplane-namespace",
		"crossplane-providerconfig",
		"crossplane-providerconfig-kind",
	}
	for _, k := range wantKeys {
		if _, ok := got[k]; !ok {
			t.Errorf("missing default key %q in %v", k, got)
		}
	}
}

func TestInitialize_CollisionDefaultWins(t *testing.T) {
	user := map[string]string{"crossplane-name": "hijacked"}
	ms := newMemoryStore("real-name", "ns", user, "pc", "ProviderConfig")
	kube := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(ms).Build()

	if err := New(kube, "metadata").Initialize(context.Background(), ms); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	if got := ms.Spec.ForProvider.Metadata["crossplane-name"]; got != "real-name" {
		t.Errorf("collision policy: crossplane-name = %q, want %q", got, "real-name")
	}
}

func TestInitialize_ObserveOnly_NoMutation(t *testing.T) {
	ms := newMemoryStore("ms", "ns", nil, "pc", "ProviderConfig")
	ms.Spec.ManagementPolicies = []xpv1.ManagementAction{xpv1.ManagementActionObserve}
	kube := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(ms).Build()

	if err := New(kube, "metadata").Initialize(context.Background(), ms); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	if ms.Spec.ForProvider.Metadata != nil {
		t.Errorf("observe-only must not mutate: got %v", ms.Spec.ForProvider.Metadata)
	}
}

func TestInitialize_NoProviderConfigRef_OmitsPCKeys(t *testing.T) {
	ms := newMemoryStore("ms", "ns", nil, "", "")
	kube := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(ms).Build()

	if err := New(kube, "metadata").Initialize(context.Background(), ms); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	got := ms.Spec.ForProvider.Metadata
	if _, ok := got["crossplane-providerconfig"]; ok {
		t.Errorf("crossplane-providerconfig should be omitted: %v", got)
	}
	if _, ok := got["crossplane-providerconfig-kind"]; ok {
		t.Errorf("crossplane-providerconfig-kind should be omitted: %v", got)
	}
	if _, ok := got["crossplane-name"]; !ok {
		t.Errorf("crossplane-name should be present: %v", got)
	}
}

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

package clients

import (
	"context"
	"strings"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("add corev1 to scheme: %v", err)
	}
	return s
}

func TestResolveLocalSecretKey_Nil_ReturnsEmpty(t *testing.T) {
	kube := fake.NewClientBuilder().WithScheme(newScheme(t)).Build()

	got, err := ResolveLocalSecretKey(context.Background(), kube, nil, "ns")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestResolveLocalSecretKey_EmptyName_ReturnsEmpty(t *testing.T) {
	kube := fake.NewClientBuilder().WithScheme(newScheme(t)).Build()

	got, err := ResolveLocalSecretKey(context.Background(), kube, &xpv1.LocalSecretKeySelector{}, "ns")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestResolveLocalSecretKey_Found(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: "ns"},
		Data:       map[string][]byte{"token": []byte("s3cret")},
	}
	kube := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(secret).Build()

	ref := &xpv1.LocalSecretKeySelector{
		LocalSecretReference: xpv1.LocalSecretReference{Name: "creds"},
		Key:                  "token",
	}
	got, err := ResolveLocalSecretKey(context.Background(), kube, ref, "ns")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "s3cret" {
		t.Fatalf("expected %q, got %q", "s3cret", got)
	}
}

func TestResolveLocalSecretKey_SecretMissing(t *testing.T) {
	kube := fake.NewClientBuilder().WithScheme(newScheme(t)).Build()

	ref := &xpv1.LocalSecretKeySelector{
		LocalSecretReference: xpv1.LocalSecretReference{Name: "creds"},
		Key:                  "token",
	}
	_, err := ResolveLocalSecretKey(context.Background(), kube, ref, "ns")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `secret "creds"`) {
		t.Fatalf("error should name the secret, got %v", err)
	}
}

func TestResolveLocalSecretKey_KeyMissing(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: "ns"},
		Data:       map[string][]byte{"other": []byte("x")},
	}
	kube := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(secret).Build()

	ref := &xpv1.LocalSecretKeySelector{
		LocalSecretReference: xpv1.LocalSecretReference{Name: "creds"},
		Key:                  "token",
	}
	_, err := ResolveLocalSecretKey(context.Background(), kube, ref, "ns")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `key "token"`) {
		t.Fatalf("error should name the key, got %v", err)
	}
}

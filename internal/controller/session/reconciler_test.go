package session

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"

	v1beta1 "github.com/jonasz-lasut/provider-anthropic/apis/managedagents/v1beta1"
)

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("add corev1: %v", err)
	}
	if err := v1beta1.AddToScheme(s); err != nil {
		t.Fatalf("add v1beta1: %v", err)
	}
	return s
}

func TestResolveSessionContext_ResolvesPerResourceTokens(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ghtok", Namespace: "ns"},
		Data:       map[string][]byte{"token": []byte("ghp_x")},
	}
	kube := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(secret).Build()

	sess := &v1beta1.Session{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns"},
		Spec: v1beta1.SessionSpec{ForProvider: v1beta1.SessionParameters{
			Resources: []v1beta1.SessionResource{
				{
					Type: ptr("github_repository"),
					AuthorizationTokenSecretRef: &xpv1.LocalSecretKeySelector{
						LocalSecretReference: xpv1.LocalSecretReference{Name: "ghtok"},
						Key:                  "token",
					},
				},
				{Type: ptr("file"), FileID: ptr("file_1")},
			},
		}},
	}

	cctx, err := resolveSessionContext(context.Background(), kube, sess)
	if err != nil {
		t.Fatalf("resolveSessionContext: %v", err)
	}
	if len(cctx.ResourceTokens) != 2 {
		t.Fatalf("ResourceTokens len = %d, want 2", len(cctx.ResourceTokens))
	}
	if cctx.ResourceTokens[0] != "ghp_x" {
		t.Errorf("ResourceTokens[0] = %q, want ghp_x", cctx.ResourceTokens[0])
	}
	if cctx.ResourceTokens[1] != "" {
		t.Errorf("ResourceTokens[1] = %q, want empty", cctx.ResourceTokens[1])
	}
}

func TestResolveSessionContext_MissingSecretReturnsError(t *testing.T) {
	kube := fake.NewClientBuilder().WithScheme(newScheme(t)).Build()
	sess := &v1beta1.Session{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns"},
		Spec: v1beta1.SessionSpec{ForProvider: v1beta1.SessionParameters{
			Resources: []v1beta1.SessionResource{{
				Type: ptr("github_repository"),
				AuthorizationTokenSecretRef: &xpv1.LocalSecretKeySelector{
					LocalSecretReference: xpv1.LocalSecretReference{Name: "missing"},
					Key:                  "token",
				},
			}},
		}},
	}
	if _, err := resolveSessionContext(context.Background(), kube, sess); err == nil {
		t.Fatal("expected error for missing secret, got nil")
	}
}

func ptr[T any](v T) *T { return &v }

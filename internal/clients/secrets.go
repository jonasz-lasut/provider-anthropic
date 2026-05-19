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

package clients

import (
	"context"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xperrors "github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResolveLocalSecretKey fetches the Secret named by ref in the supplied
// namespace and returns the string value at ref.Key. If ref is nil or
// ref.Name is empty, it returns "" and a nil error — callers use this as
// the signal "no reference set" (for example, an inactive union variant).
//
// Errors:
//   - Secret not found: wrapped error mentioning the secret name.
//   - Key missing from Secret.Data: wrapped error mentioning the key.
func ResolveLocalSecretKey(
	ctx context.Context,
	kube client.Client,
	ref *xpv1.LocalSecretKeySelector,
	namespace string,
) (string, error) {
	if ref == nil || ref.Name == "" {
		return "", nil
	}

	var s corev1.Secret
	if err := kube.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: namespace}, &s); err != nil {
		return "", xperrors.Wrapf(err, "secret %q in namespace %q", ref.Name, namespace)
	}
	v, ok := s.Data[ref.Key]
	if !ok {
		return "", xperrors.Errorf("secret %q key %q not found", ref.Name, ref.Key)
	}
	return string(v), nil
}

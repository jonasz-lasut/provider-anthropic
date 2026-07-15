#!/usr/bin/env bash
# Post-process every generated CRD:
#   * stamp the Anthropic SDK version pinned in go.mod
#   * drop controller-gen's controller-gen.kubebuilder.io/version attribution annotation, which
#     otherwise churns every CRD on each controller-tools bump even when the schema is unchanged
# Invoked from apis/generate.go after controller-gen; also safe to run from repo root.
set -euo pipefail

cd "$(dirname "$0")/.."

version=$(go list -m -f '{{.Version}}' github.com/anthropics/anthropic-sdk-go)

for crd in package/crds/*.yaml; do
    yq -i ".metadata.annotations.\"anthropic.crossplane.io/sdk-version\" = \"$version\" | del(.metadata.annotations.\"controller-gen.kubebuilder.io/version\")" "$crd"
done

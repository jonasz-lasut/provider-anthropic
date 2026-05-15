#!/usr/bin/env bash
# Stamp every generated CRD with the Anthropic SDK version pinned in go.mod.
# Invoked from apis/generate.go after controller-gen; also safe to run from repo root.
set -euo pipefail

cd "$(dirname "$0")/.."

version=$(go list -m -f '{{.Version}}' github.com/anthropics/anthropic-sdk-go)

for crd in package/crds/*.yaml; do
    yq -i ".metadata.annotations.\"anthropic.crossplane.io/sdk-version\" = \"$version\"" "$crd"
done

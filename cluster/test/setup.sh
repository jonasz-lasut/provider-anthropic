#!/usr/bin/env bash
set -aeuo pipefail

echo "Running setup.sh"

if [[ -n "${UPTEST_CLOUD_CREDENTIALS:-}" ]]; then
  echo "Creating cloud credential secret..."
  ${KUBECTL} -n crossplane-system create secret generic provider-secret --from-literal=credentials="${UPTEST_CLOUD_CREDENTIALS}" --dry-run=client -o yaml | ${KUBECTL} apply -f -

  echo "Creating a default provider config..."
  cat <<EOF | ${KUBECTL} apply -f -
apiVersion: anthropic.crossplane.io/v1beta1
kind: ClusterProviderConfig
metadata:
  name: default
spec:
  credentials:
    source: Secret
    secretRef:
      name: provider-secret
      namespace: crossplane-system
      key: credentials
  identity:
    type: APIKey
EOF
fi
if [[ -n "${UPTEST_ADMIN_CREDENTIALS:-}" ]]; then
  echo "Creating admin credential secret..."
  ${KUBECTL} -n crossplane-system create secret generic provider-admin-secret --from-literal=credentials="${UPTEST_ADMIN_CREDENTIALS}" --dry-run=client -o yaml | ${KUBECTL} apply -f -

  echo "Creating the admin provider config for organization resources..."
  cat <<EOF | ${KUBECTL} apply -f -
apiVersion: anthropic.crossplane.io/v1beta1
kind: ClusterProviderConfig
metadata:
  name: admin
spec:
  credentials:
    source: Secret
    secretRef:
      name: provider-admin-secret
      namespace: crossplane-system
      key: credentials
  identity:
    type: APIKey
EOF
fi

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

# Workload identity federation for the service-account and federation
# examples, whose endpoints accept only org:admin OAuth tokens. The organization
# must already trust the cluster's OIDC issuer through an org:admin federation
# rule (one-time bootstrap in the README); the Kind cluster reuses the signing
# keypair in cluster/local/pki/, so the trust survives recreation, in CI too.
#
#   UPTEST_FEDERATION_ORGANIZATION_ID     organization UUID
#   UPTEST_FEDERATION_RULE_ID             fdrl_... rule granting org:admin to the provider
#   UPTEST_FEDERATION_SERVICE_ACCOUNT_ID  optional svac_... the rule targets
#   UPTEST_FEDERATION_AUDIENCE            optional projected-token audience (default https://api.anthropic.com)
if [[ -n "${UPTEST_FEDERATION_RULE_ID:-}" ]]; then
  : "${UPTEST_FEDERATION_ORGANIZATION_ID:?UPTEST_FEDERATION_ORGANIZATION_ID must be set together with UPTEST_FEDERATION_RULE_ID}"
  AUDIENCE="${UPTEST_FEDERATION_AUDIENCE:-https://api.anthropic.com}"

  # Projected tokens carry a jti claim and Anthropic accepts each one once, so
  # the file must rotate faster than the provider re-exchanges it. 600 s is the
  # shortest lifetime Kubernetes allows; the kubelet rotates at 80 % of it,
  # well inside the minted token's lifetime.
  echo "Mounting a projected ServiceAccount token (audience ${AUDIENCE}) into the provider..."
  ${KUBECTL} get deploymentruntimeconfig runtimeconfig-provider-anthropic -o json \
    | jq --arg aud "${AUDIENCE}" '
        .spec.deploymentTemplate.spec.template.spec.volumes =
          ((.spec.deploymentTemplate.spec.template.spec.volumes // []) | map(select(.name != "anthropic-identity"))
            + [{name: "anthropic-identity", projected: {sources: [{serviceAccountToken: {audience: $aud, expirationSeconds: 600, path: "token"}}]}}])
        | .spec.deploymentTemplate.spec.template.spec.containers |= map(
            if .name == "package-runtime" then
              .volumeMounts = ((.volumeMounts // []) | map(select(.name != "anthropic-identity"))
                + [{name: "anthropic-identity", mountPath: "/var/run/secrets/anthropic", readOnly: true}])
            else . end)
        | del(.metadata.resourceVersion, .metadata.uid, .metadata.creationTimestamp, .metadata.generation, .metadata.managedFields, .status)' \
    | ${KUBECTL} apply -f -

  echo "Waiting for the provider to roll out with the projected token..."
  DEPLOY=""
  for _ in $(seq 1 60); do
    DEPLOY=$(${KUBECTL} -n crossplane-system get deployment -o name | grep '^deployment.apps/provider-anthropic-' | head -1 || true)
    if [[ -n "${DEPLOY}" ]] && ${KUBECTL} -n crossplane-system get "${DEPLOY}" -o json | jq -e '.spec.template.spec.volumes[]? | select(.name == "anthropic-identity")' >/dev/null; then
      break
    fi
    sleep 5
  done
  [[ -n "${DEPLOY}" ]] || { echo "provider deployment not found" >&2; exit 1; }
  ${KUBECTL} -n crossplane-system rollout status "${DEPLOY}" --timeout=300s

  echo "Creating the admin-federation provider config for the federation resources..."
  SERVICE_ACCOUNT_LINE=""
  if [[ -n "${UPTEST_FEDERATION_SERVICE_ACCOUNT_ID:-}" ]]; then
    SERVICE_ACCOUNT_LINE="      serviceAccountID: ${UPTEST_FEDERATION_SERVICE_ACCOUNT_ID}"
  fi
  cat <<EOF | ${KUBECTL} apply -f -
apiVersion: anthropic.crossplane.io/v1beta1
kind: ClusterProviderConfig
metadata:
  name: admin-federation
spec:
  credentials:
    source: None
  # A rule enabled for all workspaces needs a workspace on the exchange or it
  # answers 401; the org:admin endpoints ignore the binding.
  workspaceID: default
  identity:
    type: WorkloadIdentityFederation
    federation:
      organizationID: ${UPTEST_FEDERATION_ORGANIZATION_ID}
      federationRuleID: ${UPTEST_FEDERATION_RULE_ID}
${SERVICE_ACCOUNT_LINE}
EOF
fi

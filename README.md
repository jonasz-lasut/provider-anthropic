# provider-anthropic

`provider-anthropic` is a [Crossplane](https://crossplane.io/) Provider that enables deployment
and management of resources on the [Anthropic platform](https://docs.anthropic.com) using the
[Managed Agents beta API](https://docs.anthropic.com/en/api/managed-agents-overview):

- A `ProviderConfig` / `ClusterProviderConfig` resource type that holds an API key `Secret`.
- Managed resource types that map to Anthropic platform objects — see [Resources](#resources).

## Resources

| Kind | API group | Description |
|---|---|---|
| `Agent` | `managedagents.anthropic.crossplane.io/v1beta1` | Create and update [Managed Agents](https://docs.anthropic.com/en/docs/managed-agents) |
| `Session` | `managedagents.anthropic.crossplane.io/v1beta1` | Agent sessions with environments and vaults |
| `Deployment` | `managedagents.anthropic.crossplane.io/v1beta1` | Scheduled agent runs that materialize sessions on a cron schedule |
| `Vault` | `managedagents.anthropic.crossplane.io/v1beta1` | Credential containers for agents |
| `VaultCredential` | `managedagents.anthropic.crossplane.io/v1beta1` | OAuth and static bearer tokens in a vault |
| `Environment` | `managedagents.anthropic.crossplane.io/v1beta1` | Cloud container configuration for sessions |
| `MemoryStore` | `managedagents.anthropic.crossplane.io/v1beta1` | Named stores for agent memories |
| `MemoryStoreMemory` | `managedagents.anthropic.crossplane.io/v1beta1` | Individual text memories in a store |
| `Skill` | `managedagents.anthropic.crossplane.io/v1beta1` | Reusable skill packages and their versioned file content for agents |
| `Workspace` | `organization.anthropic.crossplane.io/v1beta1` | Organization workspaces (Admin API, needs an Admin API key) |
| `WorkspaceMember` | `organization.anthropic.crossplane.io/v1beta1` | A user's membership and role in a workspace (Admin API) |
| `Invite` | `organization.anthropic.crossplane.io/v1beta1` | Invitations of users into the organization; create-only (Admin API) |
| `ExternalKey` | `organization.anthropic.crossplane.io/v1beta1` | Customer-managed encryption key configurations (Admin API, needs CMEK) |
| `ServiceAccount` | `organization.anthropic.crossplane.io/v1beta1` | Workload identities that federation rules target (Admin API, needs federation) |
| `FederationIssuer` | `organization.anthropic.crossplane.io/v1beta1` | OIDC issuers trusted for workload identity federation (Admin API, needs federation) |
| `FederationRule` | `organization.anthropic.crossplane.io/v1beta1` | Which issuer tokens may act as which service account (Admin API, needs federation) |

## Install

If you would like to install `provider-anthropic` without modifications, you may do
so using the Crossplane CLI in a Kubernetes cluster where Crossplane is installed:

```console
crossplane xpkg install provider ghcr.io/jonasz-lasut/provider-anthropic:latest
```

You may also manually install `provider-anthropic` by creating a `Provider` directly:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-anthropic
spec:
  package: ghcr.io/jonasz-lasut/provider-anthropic:latest
```

## Supply Chain Security

Released images are signed and attested (SBOM, SLSA provenance) via keyless
cosign signing. See [docs/signature-verification.md](docs/signature-verification.md) for
verification instructions.

## Developing locally

See the header of [`go.mod`](./go.mod) for the minimum supported version of Go.

Bootstrap the build submodule on a fresh clone, then start a local development environment
with Kind where Crossplane is installed:

```console
make submodules
make
make local-deploy
```

### Running locally

Run the controller locally, out-of-cluster, against the Kind cluster:

```console
make run
```

Because the controller runs outside the Kind cluster you need the API server to be reachable.
You can expose it on a well-known port:

```console
# on a separate terminal
sudo kubectl proxy --port=8081
```

See [Required configuration](#required-configuration) for how to properly set up credentials
and RBAC for the locally running controller.

### Running in-cluster

Build, package, and deploy the provider inside the Kind cluster:

```console
make local-deploy
```

See [Required configuration](#required-configuration) for how to set up credentials and RBAC.

### Required configuration

1. Create a `Secret` holding your [Anthropic API key](https://docs.anthropic.com/en/api/getting-started).
   The credential payload is a **JSON object** whose fields depend on the identity type; for the
   `APIKey` identity it must contain an `api_key` field:

    ```console
    kubectl -n crossplane-system create secret generic anthropic-credentials \
      --from-literal=credentials='{"api_key":"YOUR_ANTHROPIC_API_KEY"}'
    ```

1. Apply a `ProviderConfig` that references the secret and selects the `APIKey` identity:

    ```console
    kubectl apply -f examples/anthropic/v1beta1/providerconfig.yaml
    ```

    Or use a cluster-scoped `ClusterProviderConfig` if your managed resources span multiple namespaces:

    ```yaml
    apiVersion: anthropic.crossplane.io/v1beta1
    kind: ClusterProviderConfig
    metadata:
      name: default
    spec:
      credentials:
        source: Secret
        secretRef:
          name: anthropic-credentials
          namespace: crossplane-system
          key: credentials
      identity:
        type: APIKey
    ```

    Both kinds accept an optional `spec.workspaceID` (a `wrkspc_...` workspace ID or `default`)
    that is sent as the `anthropic-workspace-id` header on every request. Only multi-workspace
    personal or service-account API keys honour it; a key bound to one workspace always runs
    there. Use one `ProviderConfig` per workspace rather than switching the value on an existing
    one, which would re-target every managed resource that uses it.

    The `organization.anthropic.crossplane.io` resources (`Workspace`, `WorkspaceMember`, `Invite`,
    `ExternalKey`) call the
    Admin API, which rejects regular API keys. Give them their own `ProviderConfig` whose secret
    holds an [Admin API key](https://docs.anthropic.com/en/api/administration-api) (`sk-ant-admin...`)
    with the same `APIKey` identity; see
    `examples/anthropic/v1beta1/clusterproviderconfig-admin.yaml`. An admin key cannot call the
    Messages or Managed Agents APIs, so never share one `ProviderConfig` between the two families.

1. **Workload identity federation** (`ServiceAccount`, `FederationIssuer`, `FederationRule`): these
   endpoints accept only `org:admin` OAuth tokens, never API keys. With
   `spec.identity.type: WorkloadIdentityFederation` the provider exchanges its own projected
   Kubernetes ServiceAccount token for short-lived Anthropic access tokens through a federation
   rule; the SDK caches and refreshes them, and no Secret is involved:

    ```yaml
    apiVersion: anthropic.crossplane.io/v1beta1
    kind: ClusterProviderConfig
    metadata:
      name: admin-federation
    spec:
      credentials:
        source: None
      workspaceID: default
      identity:
        type: WorkloadIdentityFederation
        federation:
          organizationID: <uuid from GET /v1/organizations/me>
          federationRuleID: fdrl_...
    ```

    Federation tokens are minted for one workspace, so `spec.workspaceID` goes into the exchange
    rather than a request header. A rule enabled for all workspaces (or more than one) needs the
    binding or the exchange answers `401 Authentication failed`; the `org:admin` endpoints then
    ignore it, so `default` is fine for the organization resources.

    The token is read from `/var/run/secrets/anthropic/token` (override with
    `spec.identity.federation.tokenFile`). Mount it with a `DeploymentRuntimeConfig` referenced by
    the `Provider`, using the audience the rule matches:

    ```yaml
    apiVersion: pkg.crossplane.io/v1beta1
    kind: DeploymentRuntimeConfig
    metadata:
      name: provider-anthropic
    spec:
      deploymentTemplate:
        spec:
          selector: {}
          template:
            spec:
              containers:
                - name: package-runtime
                  volumeMounts:
                    - name: anthropic-identity
                      mountPath: /var/run/secrets/anthropic
                      readOnly: true
              volumes:
                - name: anthropic-identity
                  projected:
                    sources:
                      - serviceAccountToken:
                          audience: https://api.anthropic.com
                          expirationSeconds: 600
                          path: token
    ```

    Keep the token lifetime short: projected tokens carry a `jti` claim and Anthropic accepts each
    one once, so the file has to rotate (the kubelet does so at 80 % of the lifetime) before the
    provider re-exchanges it shortly before its minted token expires. The provider keeps one SDK
    client per federation identity for that reason. 600 s is the shortest lifetime Kubernetes
    allows.

    Anthropic has to trust the cluster first, once, by hand: create a federation issuer for the
    cluster's OIDC issuer (`kubectl get --raw /.well-known/openid-configuration`; use `inline` keys
    from `kubectl get --raw /openid/v1/jwks` when Anthropic cannot reach the cluster, Kind
    included), a service account with the `admin` organization role, and a federation rule that
    targets it with `oauth_scope: org:admin`, `match.subject_prefix:
    system:serviceaccount:crossplane-system:provider-anthropic-*`, and `match.audience` equal to
    the projected token's audience. The API refuses `org:admin` for OAuth callers, so that first
    rule is created from a Console session. Every further issuer, rule, and `developer`-role
    service account can then be managed with the resources above.

    The local Kind cluster keeps that trust across recreations: `make controlplane.up` (and
    everything built on it) creates the cluster with a ServiceAccount signing keypair generated
    once into `cluster/local/pki/` (gitignored; the private key can mint tokens the organization
    trusts, so share it only with CI), so the issuer URL and JWKS stay the same after
    `make controlplane.down`. The E2E workflow restores the same key from the `KIND_SA_KEY`
    repository secret, so one bootstrap serves local runs and CI alike. Delete
    `cluster/local/pki/` to rotate the key, then update the issuer's inline keys and the secret.

1. **RBAC — managed resources**: If the provider is running inside the cluster (e.g. installed
   with Crossplane or via `make local-deploy`), Crossplane manages the provider's service account
   and automatically generates RBAC for its own CRDs. No manual role binding is required in this case.

   If the provider is running **outside** the cluster (e.g. `make run`), bind the provider's
   service account to a permissive role so it can reconcile its own resources:

    ```console
    SA=$(kubectl -n crossplane-system get sa -o name | grep provider-anthropic | \
         sed -e 's|serviceaccount\/|crossplane-system:|g')
    kubectl create clusterrolebinding provider-anthropic-admin-binding \
      --clusterrole cluster-admin \
      --serviceaccount="${SA}"
    ```

1. You can now create managed resources with a provider reference. See the generated examples
   under [`examples/`](./examples/):

    ```console
    kubectl create -f examples/managedagents/v1beta1/agent.yaml
    ```

### Running end-to-end tests

`make e2e` builds and deploys the provider locally, then runs the full
[Uptest](https://github.com/crossplane/uptest) end-to-end suite against the examples listed in
`UPTEST_EXAMPLE_LIST`. Set `UPTEST_CLOUD_CREDENTIALS` to the JSON credential payload
(`{"api_key":"..."}`) before running:

```console
UPTEST_CLOUD_CREDENTIALS='{"api_key":"YOUR_ANTHROPIC_API_KEY"}' \
UPTEST_EXAMPLE_LIST="examples/managedagents/v1beta1/agent.yaml" \
make e2e
```

The `organization.anthropic.crossplane.io` examples additionally need `UPTEST_ADMIN_CREDENTIALS`,
the same JSON payload holding an Admin API key, from which `cluster/test/setup.sh` creates the
`admin` ClusterProviderConfig (in CI it comes from the repository secret of the same name):

```console
UPTEST_ADMIN_CREDENTIALS='{"api_key":"YOUR_ANTHROPIC_ADMIN_API_KEY"}' \
UPTEST_EXAMPLE_LIST="examples/organization/v1beta1/workspace.yaml" \
make e2e
```

`WorkspaceMember` and `Invite` also need a datasource file: `${data.anthropic_user_id}` is the
`user_...` ID of an organization member with the `user` or `developer` role (organization admins
and billing members are implicit members of every workspace; list members with
`GET /v1/organizations/users` using the admin key), and `${data.anthropic_invite_email}` is a
throwaway address that receives the invite email during the test:

```console
mkdir -p .work && printf 'anthropic_user_id: user_...\nanthropic_invite_email: e2e@example.com\n' > .work/uptest-datasource.yaml
UPTEST_ADMIN_CREDENTIALS='{"api_key":"YOUR_ANTHROPIC_ADMIN_API_KEY"}' \
UPTEST_DATASOURCE_PATH=.work/uptest-datasource.yaml \
UPTEST_EXAMPLE_LIST="examples/organization/v1beta1/workspacemember.yaml,examples/organization/v1beta1/invite.yaml" \
make e2e
```

In CI the datasource comes from the `UPTEST_DATASOURCE` repository secret, which holds the whole
YAML file. `ExternalKey` is excluded from E2E (`upjet.upbound.io/manual-intervention`): it needs
CMEK enabled for the organization and a real KMS key.

The federation examples (`serviceaccount.yaml`, `federationissuer.yaml`, `federationrule.yaml`)
need an organization that trusts the cluster's OIDC issuer through an `org:admin` rule created in
the Console (a one-time bootstrap, see the workload identity federation section above). The Kind
cluster signs tokens with the persistent keypair in `cluster/local/pki/`, so that trust survives
`make controlplane.down` and extends to CI, which restores the same private key from the
`KIND_SA_KEY` repository secret. With the rule in place, `cluster/test/setup.sh` mounts a projected
token into the provider and creates the `admin-federation` ClusterProviderConfig:

```console
UPTEST_FEDERATION_ORGANIZATION_ID=<org uuid> \
UPTEST_FEDERATION_RULE_ID=fdrl_... \
UPTEST_FEDERATION_SERVICE_ACCOUNT_ID=svac_... \
UPTEST_EXAMPLE_LIST="examples/organization/v1beta1/serviceaccount.yaml" \
make e2e
```

In CI the three values come from repository secrets of the same names. Projected tokens carry a
`jti` claim that Anthropic accepts once, so the provider keeps one SDK client per federation
identity and the token volume rotates every 10 minutes, well inside the minted token's lifetime.
A restarted provider may hit a `jti_reused` rejection until the next rotation; the reconciler
retries and recovers on its own.

### Verifying before a pull request

Run `make reviewable` to execute all checks — code generation, formatting, and linting — and
confirm the working tree is clean before opening a PR:

```console
make reviewable
```

### Cleanup

To delete the local Kind cluster:

```console
make controlplane.down
```

## Development workflows

This repository ships [Claude Code](https://claude.ai/code) slash commands in
[`.claude/commands/`](./.claude/commands/) that automate the most common authoring tasks.
Open the repo in Claude Code and run any of the following:

| Command | Argument | What it does |
|---|---|---|
| `/add-resource` | `ResourceName[,ResourceName…]` | Scaffolds a new Crossplane managed resource from the Anthropic SDK: types file, reconciler, controller wiring, and code generation |
| `/generate-examples` | — | Regenerates the example manifests under `examples/` |
| `/update-anthropic-sdk` | — | Bumps the `anthropic-sdk-go` dependency and stamps the new version through the codebase |

Each command embeds step-by-step instructions and runs generation (`make generate`) and
deployment verification (`make local-deploy`) automatically.

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
    kubectl apply -f examples-generated/anthropic/v1beta1/providerconfig.yaml
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
   under [`examples-generated/`](./examples-generated/):

    ```console
    kubectl create -f examples-generated/managedagents/v1beta1/agent.yaml
    ```

### Running end-to-end tests

`make e2e` builds and deploys the provider locally, then runs the full
[Uptest](https://github.com/crossplane/uptest) end-to-end suite against the examples listed in
`UPTEST_EXAMPLE_LIST`. Set `UPTEST_CLOUD_CREDENTIALS` to the JSON credential payload
(`{"api_key":"..."}`) before running:

```console
UPTEST_CLOUD_CREDENTIALS='{"api_key":"YOUR_ANTHROPIC_API_KEY"}' \
UPTEST_EXAMPLE_LIST="examples-generated/managedagents/v1beta1/agent.yaml" \
make e2e
```

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
| `/generate-examples` | — | Regenerates the example manifests under `examples-generated/` |
| `/update-anthropic-sdk` | — | Bumps the `anthropic-sdk-go` dependency and stamps the new version through the codebase |

Each command embeds step-by-step instructions and runs generation (`make generate`) and
deployment verification (`make local-deploy`) automatically.

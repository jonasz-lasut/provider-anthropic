# provider-anthropic

`provider-anthropic` is a [Crossplane](https://crossplane.io/) Provider that enables deployment
and management of resources on the [Anthropic platform](https://docs.anthropic.com) using the
[Managed Agents beta API](https://docs.anthropic.com/en/api/managed-agents-overview):

- A `ProviderConfig` / `ClusterProviderConfig` resource type that holds an API key `Secret`.
- Managed resource types that map to Anthropic platform objects — see [Resources](#resources).
- `Observed<Resource>Collection` resource types that materialise one read-only managed resource
  per item returned by the Anthropic `List` API.

## Resources

| Kind | API group | Description |
|---|---|---|
| `Agent` | `managedagents.anthropic.crossplane.io/v1beta1` | Create and update [Managed Agents](https://docs.anthropic.com/en/docs/managed-agents) |
| `Session` | `managedagents.anthropic.crossplane.io/v1beta1` | Agent sessions with environments and vaults |
| `Vault` | `managedagents.anthropic.crossplane.io/v1beta1` | Credential containers for agents |
| `VaultCredential` | `managedagents.anthropic.crossplane.io/v1beta1` | OAuth and static bearer tokens in a vault |
| `Environment` | `managedagents.anthropic.crossplane.io/v1beta1` | Cloud container configuration for sessions |
| `MemoryStore` | `managedagents.anthropic.crossplane.io/v1beta1` | Named stores for agent memories |
| `MemoryStoreMemory` | `managedagents.anthropic.crossplane.io/v1beta1` | Individual text memories in a store |
| `ObservedAgentCollection` | `managedagents.anthropic.crossplane.io/v1beta1` | Observe-only collection of remote agents |

## Filtering collections with predicates

Every `Observed<Resource>Collection` spec accepts an optional `predicates` block that limits which
remote resources are materialised as child managed resources. Predicates are evaluated after the
Anthropic `List` call returns; fields that have a server-side equivalent are forwarded as query
parameters, and the remaining predicates are applied client-side.

| Field | Type | Description |
|---|---|---|
| `createdAtGte` | RFC 3339 timestamp | Include only resources created at or after this time (inclusive) |
| `createdAtLte` | RFC 3339 timestamp | Include only resources created at or before this time (inclusive) |
| `metadataMatch` | `map[string]string` | Include only resources whose API `metadata` map contains every specified key-value pair |
| `celFilter` | CEL expression string | Include only resources for which this boolean expression evaluates to `true` |

### Metadata filter

```yaml
spec:
  predicates:
    metadataMatch:
      environment: production
      team: platform
```

Only agents whose Anthropic `metadata` map contains both `environment=production` and
`team=platform` are materialised. Resources with no `metadata` field, or missing any key, are
excluded. An empty `metadataMatch` map passes all resources.

### CEL filter

The `celFilter` field accepts any [CEL](https://cel.dev) expression that returns a boolean. The
expression receives the JSON-decoded API response item as the variable `atProvider` (`map[string]any`).
Use bracket notation for field access:

```yaml
spec:
  predicates:
    celFilter: 'atProvider.name.startsWith("prod-") && atProvider.metadata.tier == "critical"'
```

CEL gives you full boolean logic, string functions, and comparisons across any field returned by
the API — use it when `metadataMatch` alone is not expressive enough. For keys that contain
special characters such as hyphens, fall back to bracket notation: `atProvider.metadata["my-key"]`.

### Combining predicates

All predicates are ANDed: a resource must satisfy every non-empty predicate to be included.

```yaml
spec:
  predicates:
    createdAtGte: "2025-01-01T00:00:00Z"
    metadataMatch:
      environment: production
    celFilter: 'atProvider.name.startsWith("prod-")'
```

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
    kubectl apply -f examples-generated/anthropic/v1alpha1/providerconfig.yaml
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

1. **RBAC — `Observed<Resource>Collection` resources**: Each `Observed<Resource>Collection`
   controller materialises child managed resources by applying Server-Side Apply patches and
   deleting stale children. The provider's service account therefore needs **create, patch, and
   delete** access to the child resource kind in addition to the collection kind itself.

   When the provider package is installed through Crossplane, the package manager derives this
   RBAC from the packaged CRDs automatically. When running locally you can apply the following
   `ClusterRole` and bind it to the provider's service account:

    ```yaml
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    metadata:
      name: provider-anthropic:observed-collection-manager
    rules:
    - apiGroups: ["managedagents.anthropic.crossplane.io"]
      resources:
        - agents
        - agents/status
      verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
    - apiGroups: ["managedagents.anthropic.crossplane.io"]
      resources:
        - observedagentcollections
        - observedagentcollections/status
      verbs: ["get", "list", "watch", "update", "patch"]
    ```

    Apply the role and bind it:

    ```console
    kubectl apply -f <above-manifest.yaml>
    SA=$(kubectl -n crossplane-system get sa -o name | grep provider-anthropic | \
         sed -e 's|serviceaccount\/|crossplane-system:|g')
    kubectl create clusterrolebinding provider-anthropic-collection-binding \
      --clusterrole provider-anthropic:observed-collection-manager \
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
| `/add-collection` | `ResourceName[,ResourceName…]` | Adds a new `Observed<Resource>Collection` CRD and reconciler that materialises read-only children from the Anthropic `List` API |
| `/generate-examples` | — | Regenerates the example manifests under `examples-generated/` |
| `/update-anthropic-sdk` | — | Bumps the `anthropic-sdk-go` dependency and stamps the new version through the codebase |

Each command embeds step-by-step instructions and runs generation (`make generate`) and
deployment verification (`make local-deploy`) automatically.

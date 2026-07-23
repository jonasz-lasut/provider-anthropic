Iterate over every managed-resource CRD in `package/crds/` and write feasible example YAML manifests to `examples/<short-group>/<version>/<kind>.yaml`, suitable for E2E testing.

`<short-group>` is the first DNS label of `spec.group` (everything before the first `.`), matching the layout of `apis/` in the repo. For example `managedagents.anthropic.crossplane.io` → `beta`, `config.anthropic.crossplane.io` → `config`.

Each generated file is self-contained: cross-resource references use `*IDSelector` with `matchLabels`, and any referenced resource is inlined into the same file as an additional YAML document, so `kubectl apply -f <one-file>` is enough to drive that resource's reconcile loop end-to-end.

## Step 1 — Identify CRDs to process

```bash
ls package/crds/*.yaml
```

Exclude:
- `*_providerconfigusages.yaml` — created by Crossplane at runtime, not user-applied.

Process everything else, including `ProviderConfig`.

For each remaining CRD, read it and record:
- `spec.group` (e.g. `managedagents.anthropic.crossplane.io`) — derive `<short-group>` as the substring before the first `.` (e.g. `beta`). This is the first output subdirectory.
- `spec.versions[*].name` where `served: true` (e.g. `v1alpha1`) — generate one example per served version, each into its own version subdirectory
- `spec.names.kind` (e.g. `Agent`) — used for `kind:`
- `spec.names.singular` (e.g. `agent`) — used for the file name `<singular>.yaml`

## Step 2 — Ensure the output tree exists

Update `examples/` in place. Do NOT wipe it.

```bash
mkdir -p examples
```

For each `(short-group, version)` you'll write to in Step 3, create that subdirectory if it does not already exist:

```bash
mkdir -p examples/<short-group>/<version>
```

If a file at the target path already exists, overwrite it. Leave unrelated files in `examples/` alone — the operator may have hand-edited or added files there.

## Step 3 — Generate one example YAML per (CRD, served version)

For each `(group, version, kind)` from Step 1, write `examples/<short-group>/<version>/<singular>.yaml`.

Read the CRD's `spec.versions[<version>].schema.openAPIV3Schema` directly with the Read tool. Walk the schema and emit a manifest that satisfies all `required:` constraints and respects every `enum`, `minLength`, `maxLength`, `minimum`, `maximum`, `pattern`, `minItems`, and `maxItems` rule.

### Top-level scaffolding (always emit)

The `apiVersion` always uses the FULL group from `spec.group` — only the directory path uses `<short-group>`.

```yaml
apiVersion: <group>/<version>
kind: <Kind>
metadata:
  name: example-<singular>
  namespace: crossplane-system
  labels:
    testing.upbound.io/example-name: example
  annotations:
    meta.upbound.io/example-id: <short-group>/<version>/<lowercase-kind>
spec:
  forProvider:
    # ... per the rules below
```

Every namespaced resource (managed resources and the namespaced `ProviderConfig`) MUST use `namespace: crossplane-system`. Cluster-scoped kinds (e.g. `ClusterProviderConfig`) omit `metadata.namespace` entirely.

- The `testing.upbound.io/example-name: example` label is what other examples' `*IDSelector.matchLabels` resolves against — every generated managed resource MUST carry it. The value is the literal string `example`; Kubernetes label values cannot contain `/`.
- The `meta.upbound.io/example-id` annotation carries the full path identifier of the **top-level (primary) resource of the file**, in the form `<short-group>/<version>/<lowercase-kind>` (e.g. `managedagents/v1beta1/agent`). For the primary document this is its own triple. For any **inlined CRD document** added in Step 4, the value is the SAME — it tracks the example the doc belongs to, not the doc's own kind. Non-CRD inlined docs (e.g. Kubernetes `Secret`) carry no `meta.upbound.io/example-id` annotation at all.

### Populating `spec.forProvider`

Walk the schema for `spec.properties.forProvider`:

- **Required fields** (listed in the schema's `required:`) — always populate:
  - `type: string`, no enum → use a descriptive literal that fits any `minLength`/`maxLength`/`pattern` (e.g. `name: example-agent`)
  - `type: string` with `enum:` → pick the enum value that is the most reasonable default; prefer the most permissive one (e.g. `type: unrestricted` for networking) unless the resource clearly calls for a stricter mode
  - `type: integer`/`number` → use a small value inside `minimum`/`maximum`
  - `type: boolean` → use `true` when it enables behaviour, otherwise `false`
  - Nested `object` → recurse and populate its own `required:` fields
  - Arrays → emit a single example item matching the item schema
- **Optional fields** — include a representative subset chosen to make the example useful for E2E:
  - Always include `description` and a small `metadata: { example: "true" }` map (this is the SDK-side metadata field on the spec, NOT Kubernetes metadata) if the schema allows them — they exercise harmless write paths
  - Include nested optional config blocks (e.g. Environment `config.networking`, `config.packages`) with a minimal but realistic shape
  - Skip optional fields that only duplicate what a required field already covers, and skip free-form text fields like `system` prompts (use the SDK default by omitting)
- **Cross-resource reference fields** (groups of `<other>Id` / `<other>IdRef` / `<other>IdSelector`):
  - Emit **only** `<other>IdSelector` with:
    ```yaml
    <other>IdSelector:
      matchLabels:
        testing.upbound.io/example-name: example
    ```
  - Selectors are resource-typed (the resolver only looks at the referenced Kind), so the shared label value is safe — there is only one example of each Kind in the cluster.
  - Never emit `<other>Id` or `<other>IdRef` in generated examples.
- **`anthropicDeletionPolicy`** — omit. Accept the schema default (`Archive`).
- **`managementPolicies`** — omit. Accept the schema default (`['*']`).
- **`providerConfigRef`** — omit. Accept the schema default (`{ kind: ClusterProviderConfig, name: default }`).
- **`writeConnectionSecretToRef`** — omit unless the resource is known to emit connection details.

### Special case: credential CRDs (`anthropic.crossplane.io`)

The `anthropic.crossplane.io` group contains two credential CRDs that are NOT managed resources — their spec lives at `spec.credentials`, not `spec.forProvider`:

- `ClusterProviderConfig` (cluster-scoped)
- `ProviderConfig` (namespaced)

Plus `ProviderConfigUsage`, which is skipped in Step 1.

Both credential examples share the same body:

```yaml
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

`spec.identity` is required. `APIKey` is currently the only supported identity type.

They differ only in metadata:

- **`ClusterProviderConfig`** — `metadata.name: default`, no `metadata.namespace` (cluster-scoped). This name matches the default `providerConfigRef: { kind: ClusterProviderConfig, name: default }` baked into every managed resource's schema, so applying just the CPC example is enough to wire credentials for every other example. Carry the standard `testing.upbound.io/example-name: example` label and `meta.upbound.io/example-id: anthropic/v1alpha1/clusterproviderconfig` annotation.
- **`ProviderConfig`** — `metadata.name: example-providerconfig`, `metadata.namespace: crossplane-system`. Use this name (NOT `default`) because the cluster-scoped CPC already owns the `default` name. Same standard label, annotation `meta.upbound.io/example-id: anthropic/v1alpha1/providerconfig`.

Append `---` and a second YAML document with the `Secret` (see Step 4 for the secret template) to EACH credential example. Both reference the same `anthropic-credentials/crossplane-system` Secret, so applying both files is idempotent — the Secret content is identical.

## Step 4 — Inline referenced resources for self-contained examples

For every kind that has at least one cross-resource reference, the generated file MUST end up containing every resource it transitively depends on, separated by `---`.

Algorithm, per generated example:

1. Write the primary resource as document #1.
2. For each cross-resource reference field on that resource, append `---` and a copy of the example you generated for the referenced kind in Step 3 (same `spec`, same `metadata.name`/`metadata.namespace`, same `testing.upbound.io/example-name: example` label). **Rewrite the inlined doc's `meta.upbound.io/example-id` annotation to the TOP-LEVEL file's triple** (e.g. when inlining `Vault` inside `vaultcredential.yaml`, the Vault doc's annotation becomes `managedagents/v1beta1/vaultcredential`, NOT `managedagents/v1beta1/vault`). Do not change names — every generated resource uses `example-<singular>`, which is unique by kind.
3. Recurse: if an inlined resource itself has cross-resource references, inline those too with the same annotation-rewrite rule. Stop when the file is closed under the reference relation.

After Step 4, for this provider's current CRD set:
- `agent.yaml` — `Agent` only (no refs)
- `environment.yaml` — `Environment` only (no refs)
- `session.yaml` — `Session` + `Agent` + `Environment` (Session references both)

### Secret inlining for credential CRDs

When generating `clusterproviderconfig.yaml` or `providerconfig.yaml`, append a Kubernetes `Secret` as document #2:

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: anthropic-credentials
  namespace: crossplane-system
type: Opaque
stringData:
  credentials: '{"api_key":"REPLACE_WITH_ANTHROPIC_API_KEY"}'
```

The credential value is a JSON object; for the `APIKey` identity it must contain an `api_key`
field. The placeholder API key is the only field the operator must edit before applying.

## Step 5 — Verify

Confirm the directory layout looks like this:

```
examples/
  anthropic/
    v1alpha1/
      clusterproviderconfig.yaml
      providerconfig.yaml
  beta/
    v1alpha1/
      agent.yaml
      environment.yaml
      session.yaml
```

Validate every generated file parses and matches its CRD:

```bash
for f in $(find examples -type f -name '*.yaml'); do
  echo "=== $f ==="
  kubectl --dry-run=client apply -f "$f" || echo "INVALID: $f"
done
```

If `kubectl` is unavailable or the cluster is not reachable, at minimum check every file parses as valid YAML:

```bash
find examples -name '*.yaml' -exec sh -c 'yq eval-all ".kind" "$1" > /dev/null || echo "INVALID: $1"' _ {} \;
```

## Step 6 — Report

Print a one-line summary per generated file: path, primary `kind`, and number of inlined documents. Example:

```
examples/managedagents/v1beta1/session.yaml  Session  (3 docs: Session, Agent, Environment)
```

Note any CRD that was skipped and why.

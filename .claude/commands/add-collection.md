Add a new Observed<Resource>Collection CRD that materializes Observe-only managed children for every item returned by the Anthropic SDK's List() for that resource.

Argument: $ARGUMENTS (one or more SDK resource names, e.g. "Agent" or "Agent,Environment,Vault")

When multiple resources are passed (comma- or space-separated), run **steps 1–5 for each resource in turn**, then run **step 6 exactly once at the end** for the whole batch.

## Step 1 — Understand the SDK List surface

Find the current SDK version and module cache location:
```bash
SDK_VERSION=$(grep 'anthropics/anthropic-sdk-go' go.mod | awk '{print $2}')
MODCACHE=$(go env GOMODCACHE)
SDK_DIR="$MODCACHE/github.com/anthropics/anthropic-sdk-go@${SDK_VERSION}"
```

From `$SDK_DIR/beta<lowercase-resource>.go` extract:
- **Service method**: confirm `Beta<Resource>Service.List(ctx, params, opts...)` exists. If it doesn't, this resource is not eligible for a collection — abort and report.
- **ListAutoPaging**: confirm `ListAutoPaging` is present (it should be wherever `List` is).
- **ListParams struct**: `Beta<Resource>ListParams` — list every field. The user-facing predicate surface is fixed: `apis/managedagents/v1beta1/predicates_types.go` defines a single `Predicates` struct (`CreatedAtGte`, `CreatedAtLte`) shared by every collection kind. For this resource, classify each `Predicates` field as either:
  - **Server-side supported**: the SDK's `Beta<Resource>ListParams` has an equivalent field — map it in `buildListParams`.
  - **Server-side unsupported**: the SDK has no equivalent — apply it client-side in `clientSideFilter`.
  Pagination (`Limit`, `Page`) and header (`Betas`) are not exposed; we always use `ListAutoPaging` and the SDK-default beta header. Archived resources are intentionally excluded from the predicate surface: the Anthropic API cannot return archived items in GET calls, so a collection's children cannot observe them — never add an `IncludeArchived` predicate.
- **Response item type**: the type-parameter to `pagination.PageCursor[T]` returned by `List`. This is what `pager.Current()` returns. Often `BetaManagedAgents<Resource>` or `Beta<Resource>` — record the exact name.

## Step 2 — Create the types file

Create `apis/managedagents/v1beta1/observed<lowercase-resource>collection_types.go`.

**Start from the template**: read `hack/collection_resource_types.go.txt` and substitute every `<Resource>` with the resource name and every `<short>` with a short name (e.g. `obscoll-agent`).

The generic `Predicates` type (`CreatedAtGte`, `CreatedAtLte`) lives in `apis/managedagents/v1beta1/predicates_types.go` and is referenced directly by `spec.Predicates`. Do **not** declare a per-resource `<Resource>Predicates` struct — the template already references the shared `Predicates` type.

**Project convention reminder:** `,omitempty` is only allowed on types with a nil representation (pointers, slices, maps). Non-pointer scalar types like `string` must be declared as `*string` before adding `,omitempty`. The template's `ObservedCollectionMember.Name` and `.ID` are non-pointer strings without `,omitempty` — keep them that way.

**Important — shared types:** `ObservedCollectionTemplate`, `ObservedCollectionTemplateMetadata`, and `ObservedCollectionMember` are shared across every collection kind. Define them **only in the first** `observed*collection_types.go` you generate. For subsequent kinds, remove those definitions from the templated output (the references in your spec/status structs continue to compile against the existing types).

## Step 3 — Create the reconciler

Create `internal/controller/observed<lowercase-resource>collection/reconciler.go`.

**Start from the template**: read `hack/collection_reconciler.go.txt` and substitute every `<Resource>` / `<lowercase-resource>` occurrence.

Fill in the placeholders:

### Response item type
Replace `*anthropic.BetaManagedAgents<Resource>` with the actual type recorded in Step 1.

### `buildListParams`
For every `Predicates` field the SDK supports server-side (the classification from Step 1), map it onto the corresponding `Beta<Resource>ListParams` field, e.g.:
```go
if p.CreatedAtGte != nil {
    params.CreatedAtGte = param.NewOpt(p.CreatedAtGte.Time)
}
if p.CreatedAtLte != nil {
    params.CreatedAtLte = param.NewOpt(p.CreatedAtLte.Time)
}
```
Use `param.NewOpt(...)` from `github.com/anthropics/anthropic-sdk-go/packages/param` for `param.Opt[T]` fields. Some SDK versions expose `anthropic.Time(...)` / `anthropic.Bool(...)` / `anthropic.String(...)` helpers — use those if present.

### `clientSideFilter`
For every `Predicates` field the SDK does NOT support server-side, drop non-matching items here. Example: if `CreatedAtGte` had no `BetaXxxListParams` equivalent:
```go
if p.CreatedAtGte != nil && item.CreatedAt.Before(p.CreatedAtGte.Time) {
    return false
}
```
If every predicate is handled by `buildListParams`, leave the body as `return true`.

### `populateForProvider`
Copy observable fields from `item` into `p`. Look at the existing reconciler's `isUpToDate` for the resource (`internal/controller/<lowercase-resource>/reconciler.go`) for the canonical list of comparable fields and reuse it. Per project convention, `ForProvider` fields are pointers — wrap values like `name := item.Name; p.Name = &name`. At minimum, copy any string field that has CRD-level constraints (`MaxLength`, `Enum`, `Pattern`) so the child admits cleanly.

## Step 4 — Wire into setup.go

Edit `internal/controller/setup.go`:
```go
import "<module>/internal/controller/observed<lowercase-resource>collection"

// in SetupProviders:
if err := observed<lowercase-resource>collection.SetupGated(mgr, o); err != nil {
    return err
}
```

## Step 5 — Generate an example manifest

Create `examples-generated/managedagents/v1beta1/observed<lowercase-resource>collection.yaml`:
```yaml
---
apiVersion: managedagents.anthropic.crossplane.io/v1beta1
kind: Observed<Resource>Collection
metadata:
  name: example-observed-<lowercase-resource>s
  namespace: crossplane-system
  labels:
    testing.upbound.io/example-name: example
  annotations:
    meta.upbound.io/example-id: managedagents/v1beta1/observed<lowercase-resource>collection
spec:
  predicates: {}
```

Optionally include one or more `Predicates` fields (`createdAtGte`, `createdAtLte`) in the example to demonstrate filtering.

## Step 6 — Regenerate and verify (run ONCE after all resources are scaffolded)

```bash
# 1. Generate DeepCopy and CRD YAML
make generate

# 2. Verify provider package deployment and build
make local-deploy
```

Fix any compilation errors. Verify the generated CRD YAML in `package/crds/` has `scope: Namespaced` for each new `observed<lowercase-resource>collections.managedagents.anthropic.crossplane.io`.

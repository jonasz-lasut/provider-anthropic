Add a new Crossplane managed resource backed by the Anthropic beta SDK.

Argument: $ARGUMENTS (one or more SDK resource names, e.g. "Environment" or "Environment,MemoryStore,File")

When multiple resources are passed (comma- or space-separated), run **steps 1–6 for each resource in turn**, then run **step 7 exactly once at the end** for the whole batch. Regeneration and `local-deploy` are repository-wide and there's no benefit to running them per-resource — running them once at the end also surfaces inter-resource issues (e.g. cross-references between newly added types) in a single pass.

## Step 1 — Understand the SDK surface

Find the current SDK version and module cache location:
```bash
SDK_VERSION=$(grep 'anthropics/anthropic-sdk-go' go.mod | awk '{print $2}')
MODCACHE=$(go env GOMODCACHE)
SDK_DIR="$MODCACHE/github.com/anthropics/anthropic-sdk-go@${SDK_VERSION}"
```

Read the SDK file for the resource:
```bash
cat "$SDK_DIR/beta<lowercase-resource>.go"
```

From the SDK file, extract:
- **Service type**: `Beta<Resource>Service` — list all exported methods (New, Get, Update, List, Archive, Delete)
- **Response type**: the struct returned by Get/New (e.g. `BetaManagedAgents<Resource>` or `Beta<Resource>`)
- **NewParams struct**: `Beta<Resource>NewParams` — all fields with their types and `api:"required"` tags
- **UpdateParams struct**: `Beta<Resource>UpdateParams` — all fields
- **ID field**: the string field in the response used as the external identifier (usually `ID string`)
- **Optimistic concurrency**: check if UpdateParams has a `Version int64` or similar field
- **Archived status**: check if the response has `ArchivedAt time.Time` (nullable)
- **Deletion methods**: does the service have `Archive()`? `Delete()`? Both? Record this.
- **Cross-resource references**: any param field named `<Other>ID string` where `<Other>` is another managed resource kind already in `apis/beta.anthropic.crossplane.io/v1alpha1/`. List these.

## Step 2 — Determine deletion policy

- **Archive only** (no Delete method on service): deletion always calls Archive. No extra spec field.
- **Delete only** (no Archive method): deletion always calls Delete. No extra spec field.
- **Both Archive and Delete**: add `AnthropicDeletionPolicy *string` to `<Resource>Parameters` (ForProvider) with:
  ```go
  // +optional
  // +kubebuilder:validation:Enum=Archive;Delete
  // +kubebuilder:default=Archive
  AnthropicDeletionPolicy *string `json:"anthropicDeletionPolicy,omitempty"`
  ```
  In the reconciler `Delete()`, read this field and call the appropriate SDK method.

## Step 3 — Create the types file

Create `apis/beta.anthropic.crossplane.io/v1alpha1/<lowercase-resource>_types.go`.

**Start from the template**: Read `hack/resource_types.go.txt` and substitute every `<Resource>` occurrence with the actual resource name (and `<plural>` / `<short>` with the plural form and short name). This gives you the license header, package declaration, imports, Spec/Status structs, kubebuilder markers, main struct, List struct, `var` block, and `init()` — all correct and ready to fill in.

Then fill in `<Resource>Parameters` (ForProvider) and `<Resource>Observation` (AtProvider) from the SDK surface found in Step 1. Key rules:
- All resources are **namespace-scoped**: `+kubebuilder:resource:scope=Namespaced`
- `<Resource>Spec` embeds `v2.ManagedResourceSpec \`json:",inline"\`` from `v2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"`. Do NOT embed `xpv1.ResourceSpec` and do NOT manually declare the three spec fields. angryjet recognises the `v2.ManagedResourceSpec` embedding and auto-generates all `resource.ModernManaged` methods, `GetItems()`, and reference resolvers — **do not write these by hand**.
- `<Resource>Status` embeds `xpv1.ResourceStatus` (keep this — it provides status conditions).

### ForProvider (`<Resource>Parameters`)
Map each SDK NewParams/UpdateParams field:
- Required string → `string`
- `param.Opt[string]` → `*string` with `+optional`
- `map[string]string` → `map[string]string` with `+optional`
- Nested struct → define a matching Go struct in the same file
- `time.Time` or timestamps → skip (read-only, go to AtProvider)
- If deletion policy field is needed, add it here (Step 2)
- **Cross-resource references**: for each `<Other>ID` field in the params, declare the value field with markers AND the `Ref`/`Selector` fields manually. angryjet generates only the `ResolveReferences` method — it does NOT generate the field declarations:
  ```go
  // +crossplane:generate:reference:type=github.com/jonasz-lasut/provider-anthropic-platform/apis/beta/v1alpha1.<Other>
  // +crossplane:generate:reference:extractor=github.com/jonasz-lasut/provider-anthropic-platform/internal/extractors.ComputedFieldExtractor("id")
  // +optional
  <Other>ID *string `json:"<lowerOther>Id,omitempty"`

  // Reference to a <Other> to populate <lowerOther>Id.
  // +kubebuilder:validation:Optional
  <Other>IDRef *xpv1.NamespacedReference `json:"<lowerOther>IdRef,omitempty"`

  // Selector for a <Other> to populate <lowerOther>Id.
  // +kubebuilder:validation:Optional
  <Other>IDSelector *xpv1.NamespacedSelector `json:"<lowerOther>IdSelector,omitempty"`
  ```
  - `reference:type` — the Go type of the Kubernetes object being pointed to.
  - `reference:extractor` — `internal/extractors.ComputedFieldExtractor("id")` reads `status.atProvider.id` of the referenced object via `fieldpath.PaveObject`. Do NOT use upjet's `resource.ExtractParamPath("id", true)` — it only works on upjet-generated Terraform-state resources and silently returns an empty string for handwritten managed resources, causing the resolver to fail with `referenced field was empty (referenced resource may not yet be ready)` even when the referenced resource is fully Ready. `ComputedFieldExtractor` works for any Go struct because it JSON-marshals the resource and walks the resulting map by path.
  - Use `xpv1.NamespacedReference` / `xpv1.NamespacedSelector` because all managed resources here are namespace-scoped. angryjet generates `ResolveReferences` in `zz_generated.resolvers.go` that uses these fields.

**Required-marker convention:** Mark every `ForProvider` field `// +optional`
regardless of whether the Anthropic API treats it as required. Anthropic
generates resource IDs server-side, so no client-supplied field is a
true identifier. Document required-by-API nature in the field's godoc with a
`Required:` prefix. Do not add `+kubebuilder:validation:MinLength` or
`+kubebuilder:validation:MinItems`, and always include `// +optional`.
`MaxLength`/`MaxItems`/`Enum`/`Pattern` markers stay — they bound or
constrain values, they don't require them.

**`,omitempty` and nil-able types:** `,omitempty` may only appear on a type
that has a nil representation — pointers, slices, or maps. Non-pointer
scalar fields (`string`, `int64`, `bool`) must be declared as pointers
(`*string`, `*int64`, `*bool`) before adding `,omitempty`. Reconcilers
that read these fields must nil-check before dereferencing. In
`isUpToDate`, a nil pointer means "user does not care about this field" —
never report it as drift.

### AtProvider (`<Resource>Observation`)
Include all read-only response fields. Always start with `ID`:

```go
// ID is the Anthropic-assigned identifier. Also stored in the external-name
// annotation, which the reconciler uses as the primary key.
// +optional
ID *string `json:"id,omitempty"`
```

The reconciler sets `AtProvider.ID` in both `Observe()` (from the Get response) and `Create()` (from the New response), immediately after calling `meta.SetExternalName`. This mirrors the external-name in a field that other resources can reference via `internal/extractors.ComputedFieldExtractor("id")`.

Also include: CreatedAt, UpdatedAt, ArchivedAt (as `*string`), Version (as `*string` if present).

**DO add `<Other>ID *string` fields** for any cross-resource reference IDs returned by the API response — these are the observed resolved values, with no markers and no `Ref`/`Selector` companions:
```go
// +optional
<Other>ID *string `json:"<lowerOther>Id,omitempty"`
```

### kubebuilder markers on the resource struct
```go
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,anthropic},shortName=<short>
```

Do NOT add `+kubebuilder:rbac:groups=` markers — RBAC for managed resources is managed by Crossplane, not via per-type markers.

Add `init()` to register both the resource and its list type with `SchemeBuilder`. angryjet generates `GetItems()` automatically — do not write it by hand.

Add package-level vars:
```go
var (
    <Resource>Kind             = "<Resource>"
    <Resource>GroupVersionKind = GroupVersion.WithKind(<Resource>Kind)
)
```

## Step 3a — Identify SecretRef candidates

After writing `<Resource>Parameters`, scan every string-typed field you
just declared and flag candidates for conversion to a Kubernetes Secret
reference.

A field is a **candidate** when it is a `string`/`*string` and matches AT
LEAST ONE of:

- **Sensitive name** (case-insensitive substring of the Go field name):
  `token`, `secret`, `password`, `apikey`, `api_key`, `accesskey`,
  `privatekey`, `credential`.
- **Long content**: a `+kubebuilder:validation:MaxLength=N` marker with
  `N > 4096`, OR the godoc contains a phrase like `"up to <N>
  characters"` / `"<N>kB"` with `N > 4096`.

And matches NONE of these exclusions:

- Field name ends in `Endpoint`, `URL`, `URI`, `Type`, `ID`/`Id`, `Ref`,
  `Selector`, or `Name`.
- Field is the discriminator of a union (a `*string` with a
  `+kubebuilder:validation:Enum` marker).
- Field type is a struct, slice, or map.

For **each** candidate, ASK the user: `"<Field> looks like a candidate
for SecretRef conversion (reason: <which rule matched>). Convert?
[y/N]"`. DO NOT auto-convert. One question per candidate.

For each confirmed candidate, REWRITE the field in the types file:

- Type becomes `xpv1.LocalSecretKeySelector` — VALUE, not a pointer
  (see [[feedback-secret-ref-shape]]).
- JSON tag becomes `<lowerCamelName>SecretRef` with NO `omitempty`.
- Drop any `+kubebuilder:validation:MaxLength` / `+kubebuilder:validation:Pattern`
  markers. The constraint moves to godoc prose; the API enforces it
  (see [[feedback-no-client-side-validation]]).
- Update the godoc: prefix with `Required:` if API-required, name the
  ref ("references a Secret in the MR's namespace holding ... at the
  given key"), and mention any size/format limit in prose.

Example before/after:

```go
// before
// Required: Content is the UTF-8 text content of the memory. Maximum 100 kB.
// +optional
// +kubebuilder:validation:MaxLength=102400
Content *string `json:"content,omitempty"`

// after
// Required: ContentSecretRef references a Secret in the MR's namespace
// holding the UTF-8 memory content at the given key. Maximum 100 kB.
ContentSecretRef xpv1.LocalSecretKeySelector `json:"contentSecretRef"`
```

## Step 4 — Create the reconciler

Create `internal/controller/<lowercase-resource>/reconciler.go`.

**Start from the template**: Read `hack/reconciler.go.txt` and substitute every `<Resource>` / `<lowercase-resource>` occurrence with the actual resource name. This gives you the license header, package declaration, imports, error constants, `Setup`/`SetupGated`, `connector`, `Connect`, `external`, and stubs for `Observe`, `Create`, `Update`, `Delete`, `Disconnect`, `buildNewParams`, `buildUpdateParams`, and `isUpToDate`.

**Default-metadata wiring.** The template includes a `skipDefaultMetadata bool` parameter on `Setup`/`SetupGated` and conditionally registers `initializer.New(mgr.GetClient(), "metadata")` so the controller writes canonical Crossplane identifiers (`crossplane-kind`, `crossplane-name`, `crossplane-namespace`, `crossplane-providerconfig`, `crossplane-providerconfig-kind`) into `spec.forProvider.metadata`. **Keep this wiring only if `<Resource>Parameters` has a `Metadata map[string]string` field.** If your resource has no such field (e.g. observed-only collections), delete the `skipDefaultMetadata` parameter from both `Setup` and `SetupGated`, drop the conditional `if !skipDefaultMetadata { … }` block, and remove the `internal/initializer` import.

Then fill in each method using the SDK surface found in Step 1. Key rules:

### Observe
**External-name is the sole source of truth for the external resource ID.**
1. `resID := meta.GetExternalName(res)` — if equal to `res.GetName()`, return `ResourceExists: false`
2. Call SDK `Get(ctx, resID, ...)`; on 404 return `ResourceExists: false`
3. If `ArchivedAt` is non-empty/non-zero, return `ResourceExists: false`
4. Populate AtProvider — `res.Status.AtProvider.ID = &resp.ID` first, then timestamps and any `<Other>ID` fields returned by the API
5. `res.SetConditions(xpv1.Available())`; call `isUpToDate`

Cross-resource reference resolution is handled automatically before `Observe()` — no manual fetch needed.

### Create
1. Build NewParams from ForProvider (use `res.Spec.ForProvider.<Other>ID` for resolved reference IDs)
2. Call SDK `New(ctx, params)`
3. `meta.SetExternalName(res, resp.ID)` then `res.Status.AtProvider.ID = &resp.ID`

For each SecretRef field on the resource, resolve the value with
`clients.ResolveLocalSecretKey(ctx, e.kube, ref, res.GetNamespace())`
BEFORE building SDK params, propagate the error if any, and only set
the corresponding SDK field when the resolved string is non-empty.
The same call also runs in `Observe` for drift detection:

- **Write-only fields** (API never returns the value, e.g. tokens):
  skip the diff in `isUpToDate`; touching the spec is the only way to
  rotate. No `Observe` resolution needed.
- **Readable fields with a hash on the response** (e.g.
  `ContentSha256` on `MemoryStoreMemory`): resolve in `Observe`,
  compute `sha256.Sum256` of the bytes, compare to the observed hash.
- **Readable fields without a hash** (e.g. Agent `system`): resolve in
  `Observe` and string-compare to the response field — one Secret read
  per poll, but correct.

Pass the resolved string into both `buildNewParams`/`buildUpdateParams`
and `isUpToDate` as an explicit parameter so the builders stay pure.
The reconciler's `external` struct grows a `kube client.Client` field
populated in `Connect`.

### Update
1. `resID := meta.GetExternalName(res)` — if equal to `res.GetName()`, return error
2. If Version is required for optimistic concurrency and is nil, return error
3. Build UpdateParams; call SDK `Update(ctx, resID, params)`

### Delete
`resID := meta.GetExternalName(res)` — if equal to `res.GetName()`, return nil (nothing to delete).

Use the deletion stub in the template: uncomment the correct variant (Archive only / Delete only / both with `AnthropicDeletionPolicy`). Always treat 404 as success.

### isUpToDate
Compare every mutable ForProvider field against the response. Use length + key-value for maps; length + element comparison for slices.

## Step 5 — Wire into setup.go

Add the new controller to `internal/controller/setup.go`. `skipDefaultMetadata` is the bool parameter on `SetupProviders` itself — it is plumbed in from the `--skip-default-metadata` CLI flag in `cmd/provider/main.go`.

```go
import "<module>/internal/controller/<lowercase-resource>"

// in SetupProviders — if <Resource>Parameters has a Metadata map field:
if err := <lowercase-resource>.SetupGated(mgr, o, skipDefaultMetadata); err != nil {
    return err
}

// OR, if the resource has no metadata field:
if err := <lowercase-resource>.SetupGated(mgr, o); err != nil {
    return err
}
```

The module path is found in `go.mod` (`module` directive).

## Step 6 — Regenerate and verify (run ONCE after all resources are scaffolded)

If multiple resources were requested, complete steps 1–6 for every resource first, then run this step a single time. Do not regenerate between resources.

Run in order (each step depends on the previous):

```bash
# 1. Generate all dependencies
make generate

# 2. Verify provider package deployment and build
make local-deploy
```

angryjet generates:
- `zz_generated.managed.go` — all 8 `resource.ModernManaged` interface methods
- `zz_generated.managedlist.go` — `GetItems()` on list types
- `zz_generated.resolvers.go` — `ResolveReferences` and `Ref`/`Selector` fields for any `+crossplane:generate:reference` markers

Fix any compilation errors. Verify the generated CRD YAML has `scope: Namespaced` for the new resource.

Add a new Crossplane managed resource backed by the Anthropic beta SDK.

Argument: $ARGUMENTS (one or more SDK resource names, e.g. "Environment" or "Environment,MemoryStore,File")

When multiple resources are passed (comma- or space-separated), run **steps 1–6 for each resource in turn**, then run **step 7 exactly once at the end** for the whole batch. Regeneration and `local-deploy` are repository-wide and there's no benefit to running them per-resource — running them once at the end also surfaces inter-resource issues (e.g. cross-references between newly added types) in a single pass.

## Step 0 — Check for a resource-specific overlay

Before doing anything else, derive the overlay path for this resource:

```
OVERLAY_PATH="docs/overlays/<lowercase-resource>.md"
```

Check whether the file exists:

```bash
ls docs/overlays/<lowercase-resource>.md 2>/dev/null && echo "OVERLAY FOUND" || echo "no overlay"
```

**If the file exists:** read it in full now. The overlay documents every
deviation this resource requires from the standard procedure below. Overlay
instructions take precedence over the matching standard steps. Keep the
overlay content in mind while executing Steps 1–6 — wherever the overlay
contradicts a step, follow the overlay.

**If no file exists:** proceed with the standard steps unchanged.

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
  // +crossplane:generate:reference:type=github.com/jonasz-lasut/provider-anthropic/apis/beta/v1alpha1.<Other>
  // +crossplane:generate:reference:extractor=github.com/jonasz-lasut/provider-anthropic/internal/extractors.ComputedFieldExtractor("id")
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
Include **all** non-sensitive SDK response fields. Use camelCase JSON tags (Kubernetes convention), matching the ForProvider JSON tags for mutable fields so the structured diff in `isUpToDate` works automatically.

Always start with `ID`:
```go
// ID is the Anthropic-assigned identifier. Also stored in the external-name
// annotation, which the reconciler uses as the primary key.
// +optional
ID *string `json:"id,omitempty"`
```

The reconciler sets `AtProvider.ID` in both `Observe()` (from the Get response) and `Create()` (from the New response), immediately after calling `meta.SetExternalName`. This mirrors the external-name in a field that other resources can reference via `internal/extractors.ComputedFieldExtractor("id")`.

**Add all mutable ForProvider fields with matching JSON tags.** For example, if ForProvider has `Name *string json:"name,omitempty"`, AtProvider must also have `Name *string json:"name,omitempty"`. This makes `clients.IsSubsetEqual` work: it finds the key `name` in both maps and compares the values.

**Add all remaining read-only response fields**: CreatedAt, UpdatedAt, ArchivedAt (as `*string`), Version (as `*int64` if present), and any other fields the API returns.

**Type-matching rules for nested SDK types:**
- When ForProvider holds a plain string ID (e.g. `Model *string json:"model"`) but the SDK response returns a typed object (e.g. `{id, type}`), define a matching observation sub-type in AtProvider:
  ```go
  type <Resource>ModelObservation struct {
      ID *string `json:"id,omitempty"`
  }
  // In <Resource>Observation:
  Model *<Resource>ModelObservation `json:"model,omitempty"`
  ```
  In `FromAnthropicObservation`, populate it explicitly: `r.Status.AtProvider.Model = &<Resource>ModelObservation{ID: &resp.Model.ID}`.
  `clients.IsSubsetEqual` automatically handles the scalar-vs-id-object pattern.
- When ForProvider and the API both use the same slice/struct type (e.g. `MCPServers []MCPServerConfig`), reuse the ForProvider type in AtProvider — the JSON keys already match.

**Sensitive fields to exclude from AtProvider** (omit entirely, do not declare):
- Credential tokens, API keys, passwords, client secrets.
- Large content bodies backed by a SecretRef (e.g. MemoryStoreMemory.Content). These are handled via SHA-256 hash comparison in `isUpToDate`.
- For nested objects containing mixed safe/sensitive fields (e.g. VaultCredential.Auth), declare a partial observation type that contains only the non-sensitive sub-fields; `json.Unmarshal` silently ignores extra fields.

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

## Step 3b — Create the conversion file

Create `apis/beta/v1alpha1/<lowercase-resource>_conversion.go` (same package as the types file).

This file contains the stable conversion interface between the Crossplane types and the Anthropic SDK. It is tested separately from the reconciler.

### Secrets context struct

If the resource has SecretRef fields, declare a `<Resource>ConversionContext` struct in this file to carry pre-resolved secret values, plus a `ToConnectionDetails()` method that publishes them:

```go
import "github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"

type <Resource>ConversionContext struct {
    // <Field> is the resolved value from <Field>SecretRef.
    <Field> string
}

// ToConnectionDetails publishes all non-empty resolved secret values as
// Crossplane connection details so consumers can access them via
// spec.writeConnectionSecretToRef.
func (cctx *<Resource>ConversionContext) ToConnectionDetails() managed.ConnectionDetails {
    cd := managed.ConnectionDetails{}
    if cctx.<Field> != "" {
        cd["<camelCaseKey>"] = []byte(cctx.<Field>)
    }
    return cd
}
```

The camelCase key names must be stable — they become the keys in the Kubernetes Secret written by Crossplane. Use the same name as the resolved field, lowercased (e.g. `system`, `content`, `bearerToken`, `accessToken`).

Resources with no SecretRef fields need no context struct and no `ToConnectionDetails` method.

### Methods to implement

```go
// ToAnthropicNew converts ForProvider to SDK create params.
// If the resource has secrets, accept ctx *<Resource>ConversionContext.
func (r *<Resource>) ToAnthropicNew([ctx *<Resource>ConversionContext]) anthropic.Beta<Resource>NewParams

// ToAnthropicUpdate converts ForProvider to SDK update params.
// If the resource has secrets, accept ctx *<Resource>ConversionContext.
func (r *<Resource>) ToAnthropicUpdate([ctx *<Resource>ConversionContext]) anthropic.Beta<Resource>UpdateParams

// FromAnthropicObservation populates AtProvider from the SDK Get response.
// ArchivedAt is always omitted — its zero value is ambiguous and the resource
// does not exist in k8s after Delete() anyway.
func (r *<Resource>) FromAnthropicObservation(resp anthropic.<ResponseType>)
```

Create `apis/beta/v1alpha1/<lowercase-resource>_conversion_test.go` (package `v1alpha1_test`) and write tests **before** implementing the methods (TDD). Use dot import `. "github.com/jonasz-lasut/provider-anthropic/apis/beta/v1alpha1"` since the test package is external.

## Step 4 — Create the reconciler

Create `internal/controller/<lowercase-resource>/reconciler.go`.

**Start from the template**: Read `hack/reconciler.go.txt` and substitute every `<Resource>` / `<lowercase-resource>` occurrence with the actual resource name. This gives you the license header, package declaration, imports, error constants, `Setup`/`SetupGated`, `connector`, `Connect`, `external`, and stubs for `Observe`, `Create`, `Update`, `Delete`, `Disconnect`, and `isUpToDate`.

The reconciler does NOT contain `buildNewParams` or `buildUpdateParams` — those live in the conversion file as methods on the resource type.

**Default-metadata wiring.** The template includes a `skipDefaultMetadata bool` parameter on `Setup`/`SetupGated` and conditionally registers `initializer.New(mgr.GetClient(), "metadata")` so the controller writes canonical Crossplane identifiers (`crossplane-kind`, `crossplane-name`, `crossplane-namespace`, `crossplane-providerconfig`, `crossplane-providerconfig-kind`) into `spec.forProvider.metadata`. **Keep this wiring only if `<Resource>Parameters` has a `Metadata map[string]string` field.** If your resource has no such field (e.g. observed-only collections), delete the `skipDefaultMetadata` parameter from both `Setup` and `SetupGated`, drop the conditional `if !skipDefaultMetadata { … }` block, and remove the `internal/initializer` import.

Then fill in each method using the SDK surface found in Step 1. Key rules:

### Observe
**External-name is the sole source of truth for the external resource ID.**
1. `resID := meta.GetExternalName(res)` — if empty **or equal to `res.GetName()`**, return `ResourceExists: false`. Crossplane seeds external-name with the k8s object name before `Create` runs; some Anthropic APIs return 400 (not 404) for names that lack the expected ID prefix, so checking the k8s name is the reliable "not yet created" gate.
2. Call SDK `Get(ctx, resID, ...)`; on 404 return `ResourceExists: false`
3. If `ArchivedAt` is non-empty/non-zero, return `ResourceExists: false`
4. Populate AtProvider by calling the conversion method:
   ```go
   res.FromAnthropicObservation(*resp)
   ```
   `FromAnthropicObservation` (defined in the conversion file) performs explicit field-by-field assignment. `ArchivedAt` is always omitted there.

5. `res.SetConditions(xpv1.Available())`
6. If the resource has SecretRef fields, resolve them into a `convCtx` and return connection details:
   ```go
   convCtx := &betav1alpha1.<Resource>ConversionContext{<Field>: resolved}
   return managed.ExternalObservation{
       ResourceExists:    true,
       ResourceUpToDate:  isUpToDate(res, resolved),
       ConnectionDetails: convCtx.ToConnectionDetails(),
   }, nil
   ```
   Resolving in `Observe` keeps the connection secret current on every reconcile loop, even when nothing has changed.

Cross-resource reference resolution is handled automatically before `Observe()` — no manual fetch needed.

### Create
1. If the resource has SecretRef fields, resolve them into a `*<Resource>ConversionContext` (via a `resolve<Resource>Context` helper in the reconciler that calls `clients.ResolveLocalSecretKey` for each SecretRef). The reconciler's `external` struct grows a `kube client.Client` field populated in `Connect`.
2. Build params: `convCtx := &betav1alpha1.<Resource>ConversionContext{...}; params := res.ToAnthropicNew(convCtx)`
3. Call SDK `New(ctx, params)`
4. `meta.SetExternalName(res, resp.ID)` then `res.Status.AtProvider.ID = &resp.ID`
5. Return connection details: `return managed.ExternalCreation{ConnectionDetails: convCtx.ToConnectionDetails()}, nil`

For SecretRef fields and drift detection:

- **Write-only fields** (API never returns the value, e.g. tokens):
  skip the diff in `isUpToDate`; touching the spec is the only way to
  rotate. Resolve in `Observe` anyway so `ToConnectionDetails` keeps the
  connection secret up-to-date on every reconcile loop.
- **Readable fields with a hash on the response** (e.g.
  `ContentSha256` on `MemoryStoreMemory`): resolve in `Observe`,
  compute `sha256.Sum256` of the bytes, compare to the observed hash.
  Store the hash in `AtProvider.<Field>Sha256`; never store the raw value.
- **Readable fields without a hash** (e.g. Agent `system`): resolve in
  `Observe`, compute SHA-256, store in `AtProvider.<Field>Sha256`, compare
  hashes. Never store the raw value in AtProvider — it would be visible to
  anyone who can `kubectl get` the resource.

### Update
1. `resID := meta.GetExternalName(res)` — if equal to `res.GetName()`, return error
2. If Version is required for optimistic concurrency and is nil, return error
3. Resolve context (same as Create for resources with secrets): `convCtx := ...; params := res.ToAnthropicUpdate(convCtx)`; call SDK `Update(ctx, resID, params)`
4. Return connection details: `return managed.ExternalUpdate{ConnectionDetails: convCtx.ToConnectionDetails()}, nil`

### Delete
`resID := meta.GetExternalName(res)` — if equal to `res.GetName()`, return nil (nothing to delete).

Use the deletion stub in the template: uncomment the correct variant (Archive only / Delete only / both with `AnthropicDeletionPolicy`). Always treat 404 as success.

### isUpToDate
Use a structured JSON diff rather than field-by-field comparison:

```go
func isUpToDate(res *betav1alpha1.<Resource>) bool {
    fpRaw, err := json.Marshal(res.Spec.ForProvider)
    if err != nil {
        return true
    }
    apRaw, err := json.Marshal(res.Status.AtProvider)
    if err != nil {
        return true
    }
    var fp, ap map[string]any
    if err := json.Unmarshal(fpRaw, &fp); err != nil {
        return true
    }
    if err := json.Unmarshal(apRaw, &ap); err != nil {
        return true
    }
    return clients.IsSubsetEqual(fp, ap)
}
```

Key properties of this approach:
- `clients.IsSubsetEqual(desired, observed)` iterates ForProvider keys and checks them against AtProvider. Keys present in `desired` but absent from `observed` (ForProvider-only fields like `AnthropicDeletionPolicy`, SecretRefs, `Ref`/`Selector` fields) are **skipped automatically** — they have no AtProvider counterpart.
- `nil` pointer / absent `omitempty` fields serialize to nothing in the ForProvider JSON map, so they are also skipped — "user does not care about this field" semantics.
- Nested objects: `clients.IsSubsetEqual` recurses into `map[string]any` values.
- Scalar ForProvider vs object AtProvider (e.g. `model: "claude-opus"` in ForProvider vs `model: {id: "claude-opus", type: "model"}` in AtProvider): `IsSubsetEqual` handles this automatically via the scalar-vs-id-object special case.
- Map and slice fields are compared with `reflect.DeepEqual` once both sides are present.

**SecretRef drift** must be checked separately after the generic diff, since the resolved secret value is never stored in AtProvider. Always use SHA-256 — never store the raw secret value in AtProvider where it would be visible to anyone who can `kubectl get` the resource.

For any SecretRef field (whether the API returns the value or not), store a `<Field>Sha256 *string` in AtProvider and compare hashes:

```go
func isUpToDate(res *betav1alpha1.<Resource>, resolvedSecret string) bool {
    // ... JSON diff as above ...
    if resolvedSecret != "" {
        sum := sha256.Sum256([]byte(resolvedSecret))
        if res.Status.AtProvider.<Field>Sha256 == nil || hex.EncodeToString(sum[:]) != *res.Status.AtProvider.<Field>Sha256 {
            return false
        }
    }
    return true
}
```

In `FromAnthropicObservation`, populate the hash from the API response value (when the API returns the field):
```go
if resp.<Field> != "" {
    sum := sha256.Sum256([]byte(resp.<Field>))
    s := hex.EncodeToString(sum[:])
    r.Status.AtProvider.<Field>Sha256 = &s
} else {
    r.Status.AtProvider.<Field>Sha256 = nil
}
// <Field> intentionally omitted — stored only as SHA-256 for drift detection.
```

For **write-only fields** (API never returns the value), `FromAnthropicObservation` cannot populate the hash — the hash must be computed from the resolved K8s secret in `isUpToDate` and compared against whatever was last stored. If the AtProvider hash is nil (first reconcile after creation), trigger an update to ensure the stored hash reflects the current secret.

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

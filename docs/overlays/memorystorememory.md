# MemoryStoreMemory — add-resource overlay

This document describes every deviation from the standard `/add-resource` skill
that applies when implementing the `MemoryStoreMemory` managed resource. The
resource predates the overlay mechanism (introduced with `Skill`); this overlay
was backported to capture its deviations so a future `/add-resource` re-run or
`/update-anthropic-sdk` sync does not regenerate it against the flat-service
pattern.

---

## 1. Sub-resource of a parent SDK service

**Standard:** one CRD maps to one flat `Beta<Resource>Service` whose methods are
`New(ctx, params)`, `Get(ctx, resID, …)`, etc.

**MemoryStoreMemory:** the CRD maps onto `BetaMemoryStoreMemoryService`, which is
**nested under the parent `MemoryStore` service** and reached as
`client.Beta.MemoryStores.Memories`. Its methods take a **positional parent ID**.

For Step 1, do **not** expect a flat service. Read `betamemorystorememory.go` in
the SDK and note the signatures:

| CRD operation | SDK call |
|---|---|
| Create | `Beta.MemoryStores.Memories.New(ctx, memoryStoreID, params)` |
| Observe | `Beta.MemoryStores.Memories.Get(ctx, memoryID, params{MemoryStoreID, View})` |
| Update | `Beta.MemoryStores.Memories.Update(ctx, memoryID, params{MemoryStoreID, …})` |
| Delete | `Beta.MemoryStores.Memories.Delete(ctx, memoryID, params{MemoryStoreID})` |

The external-name annotation stores the **memory ID** (`mem_…`), the child ID —
not the parent.

---

## 2. Parent ID threading — positional in Create, in-params elsewhere

**Standard:** all cross-resource `<Other>ID` values travel inside the params
struct produced by `ToAnthropicNew` / `ToAnthropicUpdate`.

**MemoryStoreMemory:** the parent `memoryStoreId` is threaded inconsistently and
the reconciler — not only the conversion layer — owns it:

- **Create:** parent ID is a **positional argument** —
  `Memories.New(ctx, *m.Spec.ForProvider.MemoryStoreID, params)`. It is **not**
  set on `BetaMemoryStoreMemoryNewParams`; `ToAnthropicNew` only fills `Path` and
  `Content`.
- **Observe / Update / Delete:** the child ID (external-name) is positional and
  the parent ID rides **inside** the params struct
  (`BetaMemoryStoreMemoryGetParams{MemoryStoreID, View}`,
  `…UpdateParams{MemoryStoreID}`, `…DeleteParams{MemoryStoreID}`).
  `ToAnthropicUpdate` therefore **does** set `params.MemoryStoreID`.

---

## 3. Parent ID is a *required* cross-resource reference with reconciler guards

**Standard:** cross-resource references are optional fields resolved by angryjet
before `Observe`; the reconciler does not re-check them.

**MemoryStoreMemory:** `memoryStoreId` is the parent address — without it no API
call can be targeted. The field uses the standard reference plumbing
(`MemoryStoreIDRef` / `MemoryStoreIDSelector` +
`internal/extractors.ComputedFieldExtractor("id")` pointing at `MemoryStore`),
but **every** reconciler method nil-checks it explicitly:

- **Observe:** if `MemoryStoreID` is nil/empty → `ResourceExists: false` (let the
  reference resolve on a later loop; do not error).
- **Create / Update:** if nil/empty → return `errMissingStore`
  (`"spec.forProvider.memoryStoreId not resolved"`).
- **Delete:** if nil/empty → no-op success (cannot target the API; nothing to do).

---

## 4. `Observe` requests `view=full`

**Standard:** `Get(ctx, resID)` with no view selector.

**MemoryStoreMemory:** `Get` passes
`View: anthropic.BetaManagedAgentsMemoryViewFull` so the response carries
`ContentSha256` for content-drift comparison (see section 6).

---

## 5. Delete-only — no deletion policy field

**Standard:** when a service exposes both `Archive` and `Delete`, add an
`AnthropicDeletionPolicy` field (Step 2).

**MemoryStoreMemory:** the `Memories` service exposes **`Delete` only** (no
`Archive`). `Delete()` always calls `Memories.Delete`; there is **no**
`AnthropicDeletionPolicy` field. 404 on delete is treated as success.

---

## 6. Content is a SecretRef compared by SHA-256

This follows the standard SecretRef pattern (Steps 3a / 4) and is noted here only
for completeness:

- `ContentSecretRef xpv1.LocalSecretKeySelector` (value, not pointer); resolved in
  both `Observe` and `Create`/`Update` via `clients.ResolveLocalSecretKey`.
- `MemoryStoreMemoryConversionContext{Content string}` carries the resolved value
  and publishes it as the `content` connection detail.
- Drift: `isUpToDate(m, resolvedContent)` runs the standard `IsSubsetEqual` JSON
  diff, then compares `sha256(resolvedContent)` against the API-returned
  `AtProvider.ContentSha256`. The raw content is never stored in `AtProvider`.

---

## 7. No `Metadata` field — no default-metadata wiring

`MemoryStoreMemoryParameters` has **no** `Metadata map[string]string` field, so the
default-metadata initializer is not wired in. As in `Skill`, drop the
`skipDefaultMetadata` parameter from `Setup`/`SetupGated`, the conditional
initializer block, and the `internal/initializer` import — `setup.go` calls
`memorystorememory.SetupGated(mgr, o)` with no bool argument.

---

## Checklist for implementers

- [ ] `apis/beta/v1alpha1/memorystorememory_types.go` — `MemoryStoreID` value field
      + `MemoryStoreIDRef`/`MemoryStoreIDSelector` (reference to `MemoryStore`);
      `ContentSecretRef`; `ContentSha256` in AtProvider; **no** `AnthropicDeletionPolicy`;
      **no** `Metadata` field
- [ ] `apis/beta/v1alpha1/memorystorememory_conversion.go` — `ToAnthropicNew` omits
      parent ID (positional); `ToAnthropicUpdate` sets `params.MemoryStoreID`;
      `MemoryStoreMemoryConversionContext{Content}` + `ToConnectionDetails`
- [ ] `internal/controller/memorystorememory/reconciler.go` — calls
      `Beta.MemoryStores.Memories.*`; positional parent ID in `New`; `View=full` in
      `Get`; `errMissingStore` guards in Create/Update, no-op in Delete, and
      `ResourceExists:false` in Observe when the parent is unresolved; Delete-only;
      no `skipDefaultMetadata`
- [ ] `internal/controller/setup.go` — `memorystorememory.SetupGated(mgr, o)` (no bool arg)

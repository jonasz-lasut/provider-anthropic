# Dream — add-resource overlay

`Dream` (Anthropic `BetaDreamService`, added in `anthropic-sdk-go` v1.57.0) is a
**research-preview** asynchronous memory-consolidation job: it reads a memory
store plus a set of session transcripts and writes consolidated memories into a
new output memory store. It deviates from the standard `/add-resource` pattern
in several deliberate ways recorded below, so a future `/update-anthropic-sdk`
or `/add-resource` re-run does not "correct" them.

---

## 1. Immutable — no `Update` endpoint

`BetaDreamService` has `New`, `Get`, `List`, `Archive`, `Cancel` — **no
`Update`** and no `BetaDreamUpdateParams`. A Dream is therefore immutable after
creation.

Consequences in this provider:

- There is **no `ToAnthropicUpdate`** method in `dream_conversion.go`.
- `external.Update` is a **no-op** that returns `ExternalUpdate{}` — it exists
  only to satisfy the `managed.ExternalClient` interface.
- `Observe` always returns `ResourceUpToDate: true`. There is **no
  `isUpToDate` function** and no structured JSON diff. This is intentional:
  because no field can be pushed after creation, running the standard diff
  would risk reporting unfixable drift and spinning the reconciler on the
  no-op `Update`. Every `forProvider` field is effectively create-only.

If a future SDK adds an `Update` endpoint, revisit this and adopt the standard
`isUpToDate` structured-diff pattern.

## 2. Deletion is Archive-only

The service exposes `Archive` but **no `Delete`**. So there is **no
`AnthropicDeletionPolicy` field** and no `DeletionPolicyArchive`/`Delete`
branch — `external.Delete` always calls `Beta.Dreams.Archive` (treating 404 as
success). `Observe` treats a non-zero `ArchivedAt` as "does not exist".

## 3. `Cancel` is deliberately not modeled

`BetaDreamService.Cancel` transitions an in-progress dream to the terminal
`canceled` status. It is **one-way and irreversible** — there is no un-cancel /
resume / restart method, and `canceled` is a terminal `BetaDreamStatus`
(alongside `completed` and `failed`).

We intentionally do **not** expose Cancel, for the same reason the [session]
overlay defers create-only override fields: an imperative, irreversible action
is a poor fit for a declarative managed resource. A reversible
`canceled *bool` (à la Deployment's `paused` → `Pause`/`Unpause`) is
misleading here because setting it back to `false` could never un-cancel the
dream — that is silent, unfixable drift. To re-run a canceled dream, create a
new `Dream` (which yields a new ID).

Revisit only if a concrete use case needs programmatic cancellation; if so,
design it deliberately as an explicitly-immutable, one-way `canceled` trip with
drift documentation — not as an automatic SDK-sync addition.

## 4. No `metadata` field → no default-metadata initializer

`BetaDreamNewParams` has no metadata map, so — like `Skill` and
`MemoryStoreMemory` — the controller drops the `skipDefaultMetadata` parameter
and the `internal/initializer` wiring. `Setup`/`SetupGated` take `(mgr, o)`
only, and `setup.go` calls `dream.SetupGated(mgr, o)`.

## 5. Input unions and cross-references

`BetaDreamNewParams.Inputs` is a list of a discriminated union
(`memory_store` | `sessions`), modeled as `[]DreamInput` exactly like
`SessionResource` (Type discriminator + per-variant fields, converted by
`dreamInputToParam`).

- The `memory_store` variant's `memoryStoreId` **is** a cross-resource
  reference to a `MemoryStore` (with `memoryStoreIdRef`/`memoryStoreIdSelector`);
  angryjet generates per-element resolution
  (`ResolveReferences` loops over `Inputs[i].MemoryStoreID`).
- The `sessions` variant's `sessionIds` **is** a multi-valued cross-resource
  reference to `Session` (with `sessionIdsRefs`/`sessionIdsSelector`). angryjet
  generates nested `ResolveMultiple` resolution — a `for` loop over
  `Inputs[i].SessionIDs` — so the resolved `[]string` is populated before
  create and passed straight to the SDK by `dreamInputToParam`.

## 6. Model union → object variant only

`BetaDreamNewParamsModelUnion` is `string | {id, speed}`. `forProvider.model`
is modeled as the object form (`DreamModelConfig{id, speed}`) and conversion
always sends `OfBetaDreamModelConfig`. `speed` is an optional enum
(`standard`/`fast`); the API defaults it to `standard` when omitted.

[session]: ./session.md

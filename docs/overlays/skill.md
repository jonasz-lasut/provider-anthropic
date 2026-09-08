# Skill — add-resource overlay

This document describes every deviation from the standard `/add-resource` skill
that applies when implementing the `Skill` managed resource. Read it alongside
`docs/superpowers/specs/2026-05-23-skill-managed-resource-design.md`, which
contains the full rationale.

---

## 0. GA-shaped beta service (anthropic-sdk-go >= v1.68.0)

Since SDK v1.68.0 `client.Beta.Skills` no longer sends the `skills-2025-10-02`
beta header and returns the GA Skills shapes with `Beta`-prefixed type names
(`BetaSkill`, `BetaSkillVersion`, `BetaDeletedSkill`, `BetaDeletedSkillVersion`).
The provider stays on `client.Beta.Skills`; what changed underneath:

| Beta shape (SDK <= v1.67.0) | GA shape (SDK >= v1.68.0) | Provider mapping |
|---|---|---|
| `display_title` | `display_name` (max 255, not unique, derived from `SKILL.md` `name` when omitted) | `spec.forProvider.displayTitle` keeps its CRD name and is sent as `DisplayName`; the CRD rename is tracked in issue #119 |
| `latest_version` (epoch-microsecond string) | `latest_version_id` (`skver_...`) | `atProvider.latestVersion` removed; `atProvider.latestVersionId` populated from the Skill object and used to address versions |
| `source` string (`custom` \| `anthropic`) | `source.type` enum (`custom`, `anthropic`, `anthropic_example`, `plugin`) | `atProvider.source` holds `source.type` |
| Version `directory` field | dropped; `name` is the immutable kebab-case slug and the mount directory | `atProvider.latestVersionDirectory` removed; `atProvider.latestVersionName` doc updated |
| `created_at` / `updated_at` strings | `time.Time` | formatted as RFC 3339 like every other resource |
| `Skills.Delete` returns 400 while versions exist | deletes the Skill together with all versions | reconciler `Delete()` calls `Skills.Delete` directly, no version loop |
| deleting the only version allowed | returns 400 | not exercised: the provider never deletes versions |

The skill `name` is fixed by the first upload (frontmatter `name`, or the
enclosing directory when omitted) and every later upload must resolve to the
same value, so changing the frontmatter `name` in the referenced Secret makes
`Versions.New` fail.

---

## 1. Two SDK resources, one CRD

**Standard:** one CRD maps to one `Beta<Resource>` SDK service.

**Skill:** one CRD manages two SDK services in sequence:

| CRD operation | SDK calls |
|---|---|
| Create | `Beta.Skills.New` (creates the first version) then `Beta.Skills.Versions.Get(resp.LatestVersionID)` |
| Update | `Beta.Skills.Versions.New` only (no Skill-level update) |
| Delete | `Beta.Skills.Delete` (deletes the Skill together with all of its versions) |
| Observe | `Beta.Skills.Get` then `Beta.Skills.Versions.Get(resp.LatestVersionID)` |

The external-name annotation stores the **Skill ID** (`skl_...`).

---

## 2. Multipart file upload instead of JSON params

**Standard:** `ToAnthropicNew()` returns a JSON-serialisable `Beta<Resource>NewParams`
struct; the SDK encodes it as `application/json`.

**Skill:** both `Beta.Skills.New` and `Beta.Skills.Versions.New` take
`Files []io.Reader` and are encoded as `multipart/form-data`. The `Files` slice
is **not** produced by the conversion layer — it is assembled by the reconciler's
`skillFS` helper (see section 4) and passed directly to the SDK call.

Conversion methods only produce the non-file params:

```go
// skill_conversion.go
func (r *Skill) ToAnthropicNew() anthropic.BetaSkillNewParams
func (r *Skill) ToAnthropicNewVersion() anthropic.BetaSkillVersionNewParams
func (r *Skill) FromAnthropicSkillObservation(resp anthropic.BetaSkill)
func (r *Skill) FromAnthropicVersionObservation(resp anthropic.BetaSkillVersion)
```

There is no `ToAnthropicUpdate` — use `ToAnthropicNewVersion` instead.
`Versions.New` and `Versions.Get` both return `BetaSkillVersion`, so the
reconciler feeds either response to `FromAnthropicVersionObservation`.

---

## 3. `Update()` creates a new SkillVersion, not a resource update

**Standard:** `Update()` calls the SDK's update endpoint.

**Skill:** the Anthropic API has no update endpoint for `BetaSkill` or
`BetaSkillVersion`. `Update()` in the reconciler calls
`Beta.Skills.Versions.New(ctx, skillID, params)` and then updates
`AtProvider` version fields (via `FromAnthropicVersionObservation`) and
`AtProvider.FilesSha256`.

`displayTitle` (sent as the API's `display_name`) is effectively immutable
after creation — drift on it is not detected and changes require
delete+recreate.

---

## 4. Stateful reconciler — long-lived `skillFS`

**Standard:** the reconciler's `external` struct is stateless; a new one is
built per-reconcile in `Connect()`.

**Skill:** the reconciler's `external` struct holds a `*skillFS` reference that
is **built once at process startup** and shared across all reconcile loops.
Initialise it in `Setup`/`SetupGated` and pass it through `connector` into
`external`:

```go
type connector struct {
    kube client.Client
    fs   *skillFS        // built once in Setup, passed to every external
}

type external struct {
    client *anthropic.Client
    kube   client.Client
    fs     *skillFS
}
```

`skillFS` wraps:

```go
CacheOnReadFs(
  base  = BasePathFs(OsFs, "/tmp/skill-cache"),
  layer = MemMapFs,
  cacheTime = 0,
)
```

- `BasePathFs` — security: confines all disk I/O to `/tmp/skill-cache/`
- `CacheOnReadFs` — performance + recoverability: writes land on disk
  (persistent across SIGKILL) and in MemMapFs (fast reads)
- `cacheTime=0` — safe because cache entries are content-addressed (immutable)

---

## 5. Content-addressed FS path structure

**Standard:** no FS involvement.

**Skill:** staged files live at:

```
/tmp/skill-cache/<namespace>-<name>/<dirKey>/
```

where `dirKey` = first 8 hex characters of `sha256(canonicalEncode(Secret.Data))`.

The full SHA-256 (64 hex chars) is stored in `AtProvider.FilesSha256` for drift
detection. `dirKey` is derived from it:

```go
raw     := sha256.Sum256(canonicalEncode(secret.Data))
fullHex := hex.EncodeToString(raw[:])   // stored in AtProvider.FilesSha256
dirKey  := fullHex[:8]                  // used as FS directory component
```

A cache hit is a single `afero.Exists` check on the directory. Files within the
directory use Secret keys verbatim as relative paths
(e.g. `myskill/SKILL.md`) — this is what the Anthropic multipart upload expects.

---

## 6. Path sanitisation (no analogue in standard pattern)

Before staging any files, validate every Secret key:

```go
for key := range secret.Data {
    clean := filepath.Clean(key)
    if filepath.IsAbs(clean) || strings.HasPrefix(clean+"/", "../") {
        return fmt.Errorf("invalid path in secret key: %q", key)
    }
}
```

`BasePathFs` blocks path-traversal at the FS level; this check provides
defence-in-depth and surfaces clear errors to operators.

---

## 7. `isUpToDate` — hash comparison, not JSON subset diff

**Standard:** `isUpToDate` JSON-marshals `ForProvider` and `AtProvider` and
calls `clients.IsSubsetEqual`.

**Skill:** `ForProvider` and `AtProvider` share no comparable JSON keys
(`displayTitle` drift is not detected). Use a single hash comparison:

```go
func isUpToDate(sk *v1beta1.Skill, fullHex string) bool {
    return sk.Status.AtProvider.FilesSha256 != nil &&
        *sk.Status.AtProvider.FilesSha256 == fullHex
}
```

---

## 8. Secret reference type

**Standard:** `xpv2.LocalSecretKeySelector` — references one specific key in a Secret.

**Skill:** `xpv2.LocalSecretReference` — references a whole Secret by name.
All keys are consumed; keys become file paths, values become file content.
`xpv2.LocalSecretReference` is already available via the existing
`xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"` import —
no new import required.

---

## 9. No `ConversionContext`, no connection details

**Standard:** resources with SecretRef fields declare a `<Resource>ConversionContext`
struct and publish resolved secret values as Crossplane connection details via
`spec.writeConnectionSecretToRef`.

**Skill:** skill file content is not sensitive credential material. No
`ConversionContext` struct is needed and no connection details are published.

---

## 10. `skipDefaultMetadata` wiring

`SkillParameters` has no `Metadata map[string]string` field, so the
default-metadata initializer must **not** be wired in. Remove the
`skipDefaultMetadata` parameter from both `Setup` and `SetupGated`, drop the
conditional initializer block, and omit the `internal/initializer` import —
identical to the `MemoryStoreMemory` pattern.

---

## Checklist for implementers

- [ ] `apis/managedagents/v1beta1/skill_types.go` — `FilesSecretRef xpv2.LocalSecretReference`, no `Metadata` field
- [ ] `apis/managedagents/v1beta1/skill_conversion.go` — four methods; no `ConversionContext`
- [ ] `internal/controller/skill/fs.go` — `skillFS` struct, `newSkillFS()`, `stageFiles()`, `collectReaders()`
- [ ] `internal/controller/skill/reconciler.go` — `connector` and `external` hold `*skillFS`; no `skipDefaultMetadata`
- [ ] `internal/controller/setup.go` — `skill.SetupGated(mgr, o)` (no bool arg)
- [ ] `afero` already in `go.mod` at v1.15.0 — no dependency change needed

Update the Anthropic SDK dependency and sync all reconciler logic to match upstream changes.

Argument: $ARGUMENTS (optional target version, e.g. "v1.50.0"; if empty, resolve latest from the proxy)

## Step 1 — Record current SDK version

```bash
OLD_VERSION=$(grep 'anthropics/anthropic-sdk-go' go.mod | awk '{print $2}')
MODCACHE=$(go env GOMODCACHE)
echo "Current version: $OLD_VERSION"
echo "Module cache: $MODCACHE"
```

## Step 2 — Update the SDK

If $ARGUMENTS is non-empty, use that version. Otherwise use `@latest`.

```bash
go get github.com/anthropics/anthropic-sdk-go@<version-or-latest>
go mod tidy
```

If `go get` fails, stop and report the error to the user. Do not proceed.

Record the new version:
```bash
NEW_VERSION=$(grep 'anthropics/anthropic-sdk-go' go.mod | awk '{print $2}')
echo "New version: $NEW_VERSION"
```

## Step 3 — Identify changed SDK types used by this provider

SDK types are now split across two locations per resource:

- **`apis/managedagents/v1beta1/<resource>_conversion.go`** — holds `ToAnthropicNew`, `ToAnthropicUpdate`, `FromAnthropicObservation` and uses `Beta<Resource>NewParams`, `Beta<Resource>UpdateParams`, and the response struct (e.g. `BetaManagedAgents<Resource>` or `Beta<Resource>`)
- **`internal/controller/<resource>/reconciler.go`** — holds `Observe`, `Create`, `Update`, `Delete` and uses `Beta<Resource>GetParams`, `Beta<Resource>ArchiveParams`, `Beta<Resource>DeleteParams`, and the `*anthropic.Client` service method signatures

Read both files for each resource to get the full picture of which SDK types are referenced. Typically:
- `Beta<Resource>NewParams` / `Beta<Resource>UpdateParams` → in `_conversion.go`
- `Beta<Resource>GetParams`, `Beta<Resource>ArchiveParams`, `Beta<Resource>DeleteParams` → in reconciler
- Response struct (e.g. `BetaManagedAgents<Resource>`) → in `_conversion.go` (`FromAnthropicObservation` signature)

For each type, diff old vs new:

```bash
OLD_SDK="$MODCACHE/github.com/anthropics/anthropic-sdk-go@${OLD_VERSION}"
NEW_SDK="$MODCACHE/github.com/anthropics/anthropic-sdk-go@${NEW_VERSION}"

# Struct definition
grep -A 60 '^type <TypeName> struct' "$OLD_SDK/beta<resource>.go"
grep -A 60 '^type <TypeName> struct' "$NEW_SDK/beta<resource>.go"

# Service methods
grep '^func (r \*Beta<Resource>Service)' "$OLD_SDK/beta<resource>.go"
grep '^func (r \*Beta<Resource>Service)' "$NEW_SDK/beta<resource>.go"
```

Build a diff for each type: **added fields**, **removed fields**, **renamed fields**, **changed types**, **added/removed service methods**.

## Step 4 — Update API types

For each managed resource where the SDK types changed, update `apis/managedagents.anthropic.crossplane.io/v1beta1/<resource>_types.go`:

**Added SDK param field** → add the corresponding field to `<Resource>Parameters` (ForProvider):
- Required string → `string`
- `param.Opt[string]` / optional → `*string` with `+optional`
- `map[string]string` → `map[string]string` with `+optional`
- Nested struct → define a new Go struct in the same file

**Removed SDK param field** → remove the field from `<Resource>Parameters` and from `ToAnthropicNew`/`ToAnthropicUpdate` in `apis/managedagents/v1beta1/<resource>_conversion.go`.

**Added SDK response field** → add to `<Resource>Observation` (AtProvider) and add the corresponding assignment in `FromAnthropicObservation` in `_conversion.go`.

**Removed SDK response field** → remove from `<Resource>Observation` and remove the assignment from `FromAnthropicObservation` in `_conversion.go`.

**New Archive or Delete method added to service** → if the service now has both Archive and Delete and `AnthropicDeletionPolicy` is not yet in `<Resource>Parameters`, add it:
```go
// +optional
// +kubebuilder:validation:Enum=Archive;Delete
// +kubebuilder:default=Archive
AnthropicDeletionPolicy *string `json:"anthropicDeletionPolicy,omitempty"`
```
Update the reconciler `Delete()` accordingly.

**Removed Archive or Delete method** → remove the corresponding branch from `Delete()` and remove `AnthropicDeletionPolicy` if the choice no longer exists.

## Step 5 — Update conversion and reconciler logic

### Conversion file (`apis/managedagents/v1beta1/<resource>_conversion.go`)

This file owns all SDK param building and observation population. Make changes here first, then update the tests.

**`ToAnthropicNew` / `ToAnthropicUpdate`:**
- Add assignments for new SDK param fields sourced from ForProvider
- Remove assignments for removed fields
- If a field's type changed (e.g. `string` → `param.Opt[string]`), update the assignment to use `anthropic.String(...)` or similar SDK helper
- If a new SecretRef field was added to `<Resource>ConversionContext`, add the corresponding assignment from `ctx.<Field>`

**`FromAnthropicObservation`:**
- Add explicit `r.Status.AtProvider.<Field> = ...` assignments for new response fields
- Remove assignments for removed response fields
- Update any field type conversions if the SDK response type changed (e.g. `time.Time` → `string`)

Update `apis/managedagents/v1beta1/<resource>_conversion_test.go` to cover the added/changed fields.

### Reconciler (`internal/controller/<resource>/reconciler.go`)

The reconciler is thin and rarely needs changes. Update it only when:

**SDK call signatures changed** (`GetParams`, `ArchiveParams`, `DeleteParams`):
- Update the struct literal in `Observe()`, `Delete()` etc. to match the new parameter fields

**New SecretRef field added to the resource** (requires new `ConversionContext` field):
- Add resolution of the new secret in `resolve<Resource>Context` (the helper that builds the context struct before calling `ToAnthropicNew`/`ToAnthropicUpdate`)

**`isUpToDate`:**
- The structured JSON diff requires no changes for fields that follow the standard ForProvider/AtProvider matching JSON-tag pattern
- Only update `isUpToDate` if a new field needs special drift detection (e.g. a SecretRef with a hash comparison)

### Create / Update / Delete
- Update SDK call signatures if method parameters changed
- If a new required param was added that comes from a SecretRef, add it to `resolve<Resource>Context` and to `<Resource>ConversionContext`

## Step 6 — Verify compilation

```bash
go build ./...
go vet ./...
```

Fix all errors. Common patterns:
- `undefined: anthropic.SomeType` → the type was renamed; find the new name:
  ```bash
  grep 'type.*<keyword>' "$MODCACHE/github.com/anthropics/anthropic-sdk-go@${NEW_VERSION}"/*.go
  ```
- `too many/few arguments` → a method signature changed; read the new signature and update the call
- `cannot use X as type Y` → a field type changed; update the ForProvider/AtProvider struct and build/observe logic

## Step 7 — Regenerate manifests

```bash
make generate
```

## Step 8 - Validate code quality

```bash
make reviewable
```

## Step 9 — Report changes

Summarize:
- Old SDK version → new SDK version
- For each resource: ForProvider fields added/removed, AtProvider fields added/removed, conversion file changes (`_conversion.go`), reconciler changes
- Any deletion policy changes (Archive/Delete methods added or removed)
- Any new `<Resource>ConversionContext` fields required for new SecretRef candidates

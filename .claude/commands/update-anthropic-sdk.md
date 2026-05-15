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

For each managed resource controller in `internal/controller/` (excluding `providerconfig`), read the reconciler to find which SDK types it uses. Typically:
- `Beta<Resource>NewParams`
- `Beta<Resource>UpdateParams`
- `Beta<Resource>GetParams`, `Beta<Resource>ArchiveParams`, `Beta<Resource>DeleteParams`
- The response struct (e.g. `BetaManagedAgents<Resource>` or `Beta<Resource>`)

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

For each managed resource where the SDK types changed, update `apis/beta.anthropic.crossplane.io/v1alpha1/<resource>_types.go`:

**Added SDK param field** → add the corresponding field to `<Resource>Parameters` (ForProvider):
- Required string → `string`
- `param.Opt[string]` / optional → `*string` with `+optional`
- `map[string]string` → `map[string]string` with `+optional`
- Nested struct → define a new Go struct in the same file

**Removed SDK param field** → remove the field from `<Resource>Parameters` and from `buildNewParams`/`buildUpdateParams` in the reconciler.

**Added SDK response field** → add to `<Resource>Observation` (AtProvider).

**Removed SDK response field** → remove from `<Resource>Observation` and from `Observe()` where it is populated.

**New Archive or Delete method added to service** → if the service now has both Archive and Delete and `AnthropicDeletionPolicy` is not yet in `<Resource>Parameters`, add it:
```go
// +optional
// +kubebuilder:validation:Enum=Archive;Delete
// +kubebuilder:default=Archive
AnthropicDeletionPolicy *string `json:"anthropicDeletionPolicy,omitempty"`
```
Update the reconciler `Delete()` accordingly.

**Removed Archive or Delete method** → remove the corresponding branch from `Delete()` and remove `AnthropicDeletionPolicy` if the choice no longer exists.

## Step 5 — Update reconciler logic

For each changed reconciler in `internal/controller/<resource>/reconciler.go`:

### buildNewParams / buildUpdateParams
- Add assignments for new SDK param fields sourced from ForProvider
- Remove assignments for removed fields
- If a field's type changed (e.g. `string` → `param.Opt[string]`), update the assignment to use `anthropic.String(...)` or similar SDK helper

### Observe / isUpToDate
- Add comparisons for new ForProvider fields in `isUpToDate()`
- Remove comparisons for removed fields
- Update AtProvider population in `Observe()` to reflect added/removed response fields

### Create / Update / Delete
- Update any SDK call signatures if method parameters changed
- If a new required param was added to NewParams or UpdateParams, source it from ForProvider

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
go tool controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./apis/..."
go tool controller-gen crd paths="./apis/..." output:crd:artifacts:config=package/crds
```

## Step 8 — Report changes

Summarize:
- Old SDK version → new SDK version
- For each resource: ForProvider fields added/removed, AtProvider fields added/removed, reconcile logic changes
- Any deletion policy changes (Archive/Delete methods added or removed)

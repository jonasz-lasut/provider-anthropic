# VaultCredential — add-resource overlay

This document describes every deviation from the standard `/add-resource` skill
that applies when implementing the `VaultCredential` managed resource. The
resource predates the overlay mechanism (introduced with `Skill`); this overlay
was backported to capture its deviations so a future `/add-resource` re-run or
`/update-anthropic-sdk` sync does not regenerate it against the flat-service,
single-secret pattern.

---

## 1. Sub-resource of a parent SDK service

**Standard:** one CRD maps to one flat `Beta<Resource>Service` whose methods are
`New(ctx, params)`, `Get(ctx, resID, …)`, etc.

**VaultCredential:** the CRD maps onto `BetaVaultCredentialService`, which is
**nested under the parent `Vault` service** and reached as
`client.Beta.Vaults.Credentials`. Its methods take a **positional parent ID**.

For Step 1, do **not** expect a flat service. Read `betavaultcredential.go` in
the SDK and note the signatures:

| CRD operation | SDK call |
|---|---|
| Create | `Beta.Vaults.Credentials.New(ctx, vaultID, params)` |
| Observe | `Beta.Vaults.Credentials.Get(ctx, credentialID, params{VaultID})` |
| Update | `Beta.Vaults.Credentials.Update(ctx, credentialID, params{VaultID, …})` |
| Delete | `Beta.Vaults.Credentials.Delete(ctx, credentialID, params{VaultID})` |
| Archive | `Beta.Vaults.Credentials.Archive(ctx, credentialID, params{VaultID})` |

The external-name annotation stores the **credential ID** (`cred_…`), the child
ID — not the parent.

---

## 2. Parent ID threading — positional in Create, in-params elsewhere

**Standard:** all cross-resource `<Other>ID` values travel inside the params
struct produced by `ToAnthropicNew` / `ToAnthropicUpdate`.

**VaultCredential:** the parent `vaultId` is threaded inconsistently:

- **Create:** parent ID is a **positional argument** —
  `Credentials.New(ctx, *vc.Spec.ForProvider.VaultID, params)`; it is **not** set
  on `BetaVaultCredentialNewParams`.
- **Observe / Update / Delete / Archive:** the child ID (external-name) is
  positional and the parent `VaultID` rides **inside** the params struct.
  `ToAnthropicUpdate` therefore sets `params.VaultID`.

---

## 3. Parent ID is a *required* cross-resource reference with reconciler guards

**Standard:** cross-resource references are optional and the reconciler does not
re-check them after angryjet resolution.

**VaultCredential:** `vaultId` is the parent address. The field uses the standard
reference plumbing (`VaultIDRef` / `VaultIDSelector` +
`internal/extractors.ComputedFieldExtractor("id")` pointing at `Vault`), but every
reconciler method nil-checks it:

- **Observe:** nil/empty → `ResourceExists: false` (defer to a later loop).
- **Create / Update:** nil/empty → `errMissingVault`
  (`"spec.forProvider.vaultId not resolved"`).
- **Delete:** nil/empty → no-op success.

---

## 4. `Auth` is a discriminated union (deepest deviation)

**Standard:** a nested struct field maps to one matching Go struct.

**VaultCredential:** `spec.forProvider.auth` is a **discriminated union** keyed by
`auth.type` with three variants, two of which nest further unions:

```
auth.type ∈ { static_bearer, mcp_oauth, environment_variable }
  mcp_oauth.refresh.tokenEndpointAuth.type ∈ { none, client_secret_basic, client_secret_post }
  environment_variable.networking.type    ∈ { unrestricted, limited }
```

Conversion is **not** a flat field map — it is a set of switch-on-discriminator
helpers in `vaultcredential_conversion.go`, with parallel New/Update variants:

- `vcNewAuthUnion` / `vcUpdateAuthUnion` — top-level `auth.type` switch
- `vcNetworkingUnion` — shared networking union (create + update)
- `vcNewRefreshParams` / `vcUpdateRefreshParams` — OAuth refresh block
- `vcNewTEPUnion` / `vcUpdateTEPUnion` — token-endpoint-auth union

`FromAnthropicObservation` writes a **partial** `VaultCredentialAuthObservation`
holding only non-sensitive sub-fields (`type`, `mcpServerUrl`, or — for
`environment_variable` — `secretName` + `networking` + `injectionLocation`);
secrets are never observed.

---

## 5. Multiple SecretRefs spread across union variants

**Standard:** Step 3a converts secret fields one at a time, and the
`ConversionContext` typically carries a single resolved value.

**VaultCredential:** five SecretRefs live across the union variants, and which
ones are relevant depends on `auth.type`:

| SecretRef | Variant | Connection-detail key |
|---|---|---|
| `TokenSecretRef` | `static_bearer` | `bearerToken` |
| `AccessTokenSecretRef` | `mcp_oauth` | `accessToken` |
| `RefreshTokenSecretRef` | `mcp_oauth` (refresh) | `refreshToken` |
| `ClientSecretSecretRef` | `mcp_oauth` (refresh, `client_secret_*`) | `clientSecret` |
| `SecretValueSecretRef` | `environment_variable` | `secretValue` |

`VaultCredentialConversionContext` carries all five resolved strings;
`ToConnectionDetails` publishes each non-empty value. The reconciler helper
`resolveVCContext` resolves only the SecretRefs relevant to the active variant.
All five are **write-only** — the API never returns them — so there is **no**
SHA-256 hash drift detection; rotation happens by touching the spec.

---

## 6. Conversion methods return an error

**Standard:** `ToAnthropicNew` / `ToAnthropicUpdate` return only the params struct.

**VaultCredential:** both return `(params, error)` because union construction can
fail (e.g. unparseable inputs). The reconciler wraps and propagates the error in
Create/Update.

---

## 7. Archive + Delete deletion policy (with positional parent)

This follows the standard Step 2 deletion-policy pattern — noted only because the
parent ID must be carried in the Archive/Delete params:

- `AnthropicDeletionPolicy *string` (`Enum=Archive;Delete`, default `Archive`).
- `Delete()` switches on it and calls either
  `Credentials.Delete(ctx, credID, params{VaultID})` or
  `Credentials.Archive(ctx, credID, params{VaultID})`. 404 is success.
- `Observe` treats a non-zero `ArchivedAt` as `ResourceExists: false`.

---

## Checklist for implementers

- [ ] `apis/beta/v1alpha1/vaultcredential_types.go` — `VaultID` value field +
      `VaultIDRef`/`VaultIDSelector` (reference to `Vault`); discriminated `Auth`
      union types (networking, refresh, token-endpoint-auth); five `*SecretRef`
      fields; `AnthropicDeletionPolicy`; partial `VaultCredentialAuthObservation`
- [ ] `apis/beta/v1alpha1/vaultcredential_conversion.go` — `ToAnthropicNew`/
      `ToAnthropicUpdate` return `(params, error)`; parent ID positional in New,
      in-params in Update; union helpers (`vcNewAuthUnion`, `vcNetworkingUnion`,
      `vcNewRefreshParams`, `vcNewTEPUnion`, + Update variants);
      `VaultCredentialConversionContext` with five resolved secrets
- [ ] `internal/controller/vaultcredential/reconciler.go` — calls
      `Beta.Vaults.Credentials.*`; positional parent ID in `New`; `errMissingVault`
      guards in Create/Update, no-op in Delete, `ResourceExists:false` in Observe
      when the parent is unresolved; `resolveVCContext` helper; deletion-policy switch
- [ ] `internal/controller/setup.go` — `vaultcredential.SetupGated(mgr, o, skipDefaultMetadata)`
      (resource has a `Metadata` field)

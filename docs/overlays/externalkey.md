# ExternalKey — add-resource overlay

This document describes every deviation from the standard `/add-resource` skill
that applies when implementing the `ExternalKey` (customer-managed encryption
key, CMEK) managed resource. It lives in the `organization` group, so sections
1 and 2 of `docs/overlays/workspace.md` (separate API group, Admin-key
`ProviderConfig`) apply as well.

---

## 1. `providerConfig` is a three-variant union flattened into one struct

**Standard:** nested SDK structs map one-to-one onto Go structs.

**ExternalKey:** `provider_config` is a discriminated union
(`aws` | `gcp` | `azure`). The CRD flattens every variant's fields into one
`ExternalKeyProviderConfig` with a `type` discriminator, and the conversion
switches on `type` to fill `OfAWS`, `OfGCP`, or `OfAzure`. The API response
(`BetaExternalKeyProviderConfigUnion`) is already flat, so
`FromAnthropicObservation` copies the non-empty fields back and the structured
diff works key by key. The same struct serves ForProvider and AtProvider.

---

## 2. Delete only, parameterless Get and Delete

`Get(ctx, id)` and `Delete(ctx, id)` take no params struct, and the service
has no `Archive`, so there is no `AnthropicDeletionPolicy` field.

---

## 3. `Validate` is not wired

`ExternalKeys.Validate` runs a live KMS encrypt/decrypt round trip (up to
30 s). It is not called by the reconciler; a future change could surface it as
a condition.

---

## 4. Not E2E-testable on the maintainer's organization

The list endpoint answers `404 Resource not found.` for organizations without
CMEK enabled, and creating a key needs a real KMS key the organization can
reach. The example carries `upjet.upbound.io/manual-intervention`, so Uptest
skips it; the resource is verified by unit tests only.

---

## Checklist for implementers

- [ ] `apis/organization/v1beta1/externalkey_types.go` — `ExternalKeyProviderConfig` (type enum aws;gcp;azure), `DisplayName`, `Geo` (enum us)
- [ ] `apis/organization/v1beta1/externalkey_conversion.go` — variant switch in `ToAnthropicNew`/`ToAnthropicUpdate`, flat copy-back in `FromAnthropicObservation`
- [ ] `internal/controller/externalkey/reconciler.go` — standard pattern, parameterless `Get`/`Delete`
- [ ] `examples/organization/v1beta1/externalkey.yaml` — `upjet.upbound.io/manual-intervention` annotation

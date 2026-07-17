# TunnelCertificate — add-resource overlay

`TunnelCertificate` (Anthropic `BetaTunnelCertificateService`, added in
`anthropic-sdk-go` v1.58.0) registers a public CA certificate on a
[`Tunnel`](./tunnel.md). Anthropic verifies the gateway's server certificate
against this CA when it terminates the inner TLS session. A tunnel holds at most
two non-archived certificates and rejects MCP traffic until at least one is
present.

It combines the **sub-resource** deviations of
[`MemoryStoreMemory`](./memorystorememory.md) with the **immutable /
archive-only** deviations of [`Dream`](./dream.md). The departures from the
standard `/add-resource` pattern are recorded below so a future
`/update-anthropic-sdk` or `/add-resource` re-run does not "correct" them.

---

## 1. Sub-resource of the Tunnel service

**Standard:** one CRD maps to one flat `Beta<Resource>Service` whose methods are
`New(ctx, params)`, `Get(ctx, resID, …)`, etc.

**TunnelCertificate:** the CRD maps onto `BetaTunnelCertificateService`, which is
**nested under the parent `Tunnel` service** and reached as
`client.Beta.Tunnels.Certificates`. Its methods take the parent tunnel ID:

| CRD operation | SDK call |
|---|---|
| Create | `Beta.Tunnels.Certificates.New(ctx, tunnelID, params)` |
| Observe | `Beta.Tunnels.Certificates.Get(ctx, certID, params{TunnelID})` |
| Delete | `Beta.Tunnels.Certificates.Archive(ctx, certID, params{TunnelID})` |

The external-name annotation stores the **certificate ID** (`tcrt_…`), the child
ID — not the parent.

## 2. Parent ID threading — positional in Create, in-params elsewhere

Same shape as `MemoryStoreMemory`:

- **Create:** parent ID is a **positional argument** —
  `Certificates.New(ctx, *tc.Spec.ForProvider.TunnelID, params)`. It is **not**
  set on `BetaTunnelCertificateNewParams`; `ToAnthropicNew` only fills
  `CaCertificatePem`.
- **Observe / Delete:** the child ID (external-name) is positional and the
  parent ID rides **inside** the params struct
  (`BetaTunnelCertificateGetParams{TunnelID}`,
  `BetaTunnelCertificateArchiveParams{TunnelID}`).

## 3. Parent ID is a *required* cross-resource reference with reconciler guards

`tunnelId` is the parent address — without it no API call can be targeted. The
field uses the standard reference plumbing (`TunnelIDRef` / `TunnelIDSelector` +
`internal/extractors.ComputedFieldExtractor("id")` pointing at `Tunnel`), but
**every** reconciler method nil-checks it explicitly (as in `MemoryStoreMemory`):

- **Observe:** if `TunnelID` is nil/empty → `ResourceExists: false` (let the
  reference resolve on a later loop; do not error).
- **Create:** if nil/empty → return `errMissingTunnel`
  (`"spec.forProvider.tunnelId not resolved"`).
- **Delete:** if nil/empty → no-op success (cannot target the API; nothing to do).

## 4. Immutable — no `Update` endpoint

`BetaTunnelCertificateService` has `New`, `Get`, `List`, `Archive` — **no
`Update`**. As with `Dream` and `Tunnel`:

- There is **no `ToAnthropicUpdate`** method.
- `external.Update` is a **no-op**.
- `Observe` always returns `ResourceUpToDate: true`; there is **no
  `isUpToDate` function**. To rotate a CA, create a new `TunnelCertificate`
  (a tunnel allows two, so add-then-archive gives zero-downtime rotation).

## 5. Deletion is Archive-only

The service exposes `Archive` but **no `Delete`**. So there is **no
`AnthropicDeletionPolicy` field** — `external.Delete` always calls
`Certificates.Archive` (treating 404 as success). `Observe` treats a non-zero
`ArchivedAt` as "does not exist". Archiving the last certificate is permitted;
the tunnel then rejects MCP traffic until a new one is added.

## 6. `CaCertificatePem` is a SecretRef, but **not** a published credential

`BetaTunnelCertificateNewParams.CaCertificatePem` (a PEM CA cert, max 8 kB) is a
long (> 4 kB) string, so it is modeled as a SecretRef per the standard Step 3a
size rule:

- `CaCertificatePemSecretRef *xpv2.LocalSecretKeySelector` — integrates with
  `kubernetes.io/tls` / cert-manager Secrets (reference the `ca.crt` key).
- `TunnelCertificateConversionContext{CaCertificatePem}` carries the resolved
  value into `ToAnthropicNew`.

It differs from the sensitive-SecretRef pattern (`MemoryStoreMemory.Content`,
`VaultCredential` tokens) in two ways, because a CA certificate is **public**
input material, not a credential:

- The context has **no `ToConnectionDetails`** — there is nothing sensitive to
  publish, and the API never returns the PEM.
- The secret is resolved **only in `Create`**, never in `Observe`. Because the
  resource is immutable there is no drift to detect, so no SHA-256 hash is stored
  in `AtProvider` and no per-loop re-resolution happens.

## 7. No `metadata` field → no default-metadata initializer

`BetaTunnelCertificateNewParams` has no metadata map, so — like `Skill`,
`MemoryStoreMemory`, `Dream` and `Tunnel` — the controller drops the
`skipDefaultMetadata` parameter and the `internal/initializer` wiring.
`Setup`/`SetupGated` take `(mgr, o)` only, and `setup.go` calls
`tunnelcertificate.SetupGated(mgr, o)`.

---

## Checklist for implementers

- [ ] `apis/managedagents/v1beta1/tunnelcertificate_types.go` — `TunnelID` value
      field + `TunnelIDRef`/`TunnelIDSelector` (reference to `Tunnel`);
      `CaCertificatePemSecretRef`; AtProvider has
      `id`/`tunnelId`/`fingerprint`/`expiresAt`/`createdAt`; **no**
      `AnthropicDeletionPolicy`; **no** `Metadata` field
- [ ] `apis/managedagents/v1beta1/tunnelcertificate_conversion.go` —
      `ToAnthropicNew(ctx)` omits parent ID (positional), fills only
      `CaCertificatePem`; **no** `ToAnthropicUpdate`; **no** `ToConnectionDetails`;
      `FromAnthropicObservation` omits `archivedAt`
- [ ] `internal/controller/tunnelcertificate/reconciler.go` — `external{client,
      kube}`; `Beta.Tunnels.Certificates.*`; positional parent ID in `New`;
      `errMissingTunnel` guards in Create, no-op in Delete, `ResourceExists:false`
      in Observe when the parent is unresolved; SecretRef resolved only in Create;
      `Update` no-op; Archive-only; no `skipDefaultMetadata`
- [ ] `internal/controller/setup.go` — `tunnelcertificate.SetupGated(mgr, o)` (no bool arg)

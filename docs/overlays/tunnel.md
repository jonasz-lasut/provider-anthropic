# Tunnel — add-resource overlay

`Tunnel` (Anthropic `BetaTunnelService`, added in `anthropic-sdk-go` v1.58.0) is
a **research-preview** MCP tunnel: it allocates a fresh Anthropic-owned hostname
and provisions a tunnel that routes MCP traffic to a self-hosted gateway. It
deviates from the standard `/add-resource` pattern in several deliberate ways
recorded below, so a future `/update-anthropic-sdk` or `/add-resource` re-run
does not "correct" them.

Its child CA certificates are a separate managed resource,
[`TunnelCertificate`](./tunnelcertificate.md); a tunnel rejects MCP traffic
until at least one certificate is registered.

---

## 1. Immutable — no `Update` endpoint

`BetaTunnelService` has `New`, `Get`, `List`, `Archive`, `RevealToken`,
`RotateToken` — **no `Update`** and no `BetaTunnelUpdateParams`. A Tunnel is
therefore immutable after creation.

Consequences in this provider (identical to [Dream](./dream.md)):

- There is **no `ToAnthropicUpdate`** method in `tunnel_conversion.go`.
- `external.Update` is a **no-op** that returns `ExternalUpdate{}` — it exists
  only to satisfy the `managed.ExternalClient` interface.
- `Observe` always returns `ResourceUpToDate: true`. There is **no
  `isUpToDate` function** and no structured JSON diff. Every `forProvider`
  field (only `displayName`) is effectively create-only; editing it after
  creation is silently not reconciled.

If a future SDK adds an `Update` endpoint, revisit this and adopt the standard
`isUpToDate` structured-diff pattern.

## 2. Deletion is Archive-only

The service exposes `Archive` but **no `Delete`**. So there is **no
`AnthropicDeletionPolicy` field** and no `DeletionPolicyArchive`/`Delete`
branch — `external.Delete` always calls `Beta.Tunnels.Archive` (treating 404 as
success). `Observe` treats a non-zero `ArchivedAt` as "does not exist".

Archival is irreversible: the hostname is retired and never re-allocated, every
non-archived certificate is archived in the same operation, and the connector
token is invalidated.

## 3. The connector token is published as a connection detail

`RevealToken` returns a `BetaTunnelToken` (`{id, tunnel_token}`) — the credential
a self-hosted connector uses to run the tunnel. The value is fetched live on
each call and is **not** stored by Anthropic; it is stable until rotated.

We model it as an **output credential**, not a spec field:

- `TunnelConversionContext{Domain, TunnelToken, TokenID}` in
  `tunnel_conversion.go` carries the published values and emits them via
  `ToConnectionDetails()` as the `domain`, `tunnelToken` and `tokenId`
  connection details (consumable through `spec.writeConnectionSecretToRef`). The
  `domain` is duplicated from `status.atProvider.domain` so a consumer gets the
  hostname and the token together in one Secret.
- Unlike every other `*ConversionContext` in this provider (which carry resolved
  *input* secrets read from Kubernetes), this one carries *outputs* fetched from
  the API — the domain from the Get/New response and the token from
  `RevealToken`. `ToAnthropicNew` does **not** take the context.
- The reconciler calls `Beta.Tunnels.RevealToken` in **both `Create` and every
  `Observe`** (via the `revealToken` helper) so the published secret stays
  current. This is one extra live API call per reconcile loop — accepted to keep
  the connection secret fresh, mirroring how other resources resolve+publish
  connection details on every Observe.
- The token is **never** stored in `AtProvider` — status is world-readable to
  anyone who can `kubectl get`.

## 4. `RotateToken` is deliberately not modeled

`BetaTunnelService.RotateToken` invalidates the current token for new
connections and returns a fresh value. It is an **imperative, one-way action**
with no declarative fit — the same reasoning that defers [Dream](./dream.md)'s
`Cancel`. To rotate, call the SDK/API directly; the next `Observe` will re-reveal
and re-publish the new value automatically.

Revisit only if a concrete use case needs programmatic rotation; if so, design
it deliberately (e.g. an explicit generation counter), not as an automatic
SDK-sync addition.

## 5. No `metadata` field → no default-metadata initializer

`BetaTunnelNewParams` has only `DisplayName` (optional) and no metadata map, so
— like `Skill`, `MemoryStoreMemory` and `Dream` — the controller drops the
`skipDefaultMetadata` parameter and the `internal/initializer` wiring.
`Setup`/`SetupGated` take `(mgr, o)` only, and `setup.go` calls
`tunnel.SetupGated(mgr, o)`.

## 6. Certificates sub-service is a separate resource

`BetaTunnelService` embeds `Certificates BetaTunnelCertificateService`. That is
modeled as its own managed resource, [`TunnelCertificate`](./tunnelcertificate.md),
not as a nested field of `Tunnel`.

---

## Checklist for implementers

- [ ] `apis/managedagents/v1beta1/tunnel_types.go` — `DisplayName *string` only in
      ForProvider; AtProvider has `id`/`displayName`/`domain`/`createdAt`; **no**
      token field; **no** `AnthropicDeletionPolicy`; **no** `Metadata` field
- [ ] `apis/managedagents/v1beta1/tunnel_conversion.go` — `ToAnthropicNew` (no
      context, no update counterpart); `FromAnthropicObservation` omits token +
      `archivedAt`; `TunnelConversionContext{TunnelToken, TokenID}` +
      `ToConnectionDetails`
- [ ] `internal/controller/tunnel/reconciler.go` — `external{client}` (no kube);
      `Beta.Tunnels.*`; `RevealToken` in Create and Observe; `Observe` returns
      `ResourceUpToDate: true`; `Update` no-op; Archive-only Delete; no
      `skipDefaultMetadata`
- [ ] `internal/controller/setup.go` — `tunnel.SetupGated(mgr, o)` (no bool arg)

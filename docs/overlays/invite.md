# Invite — add-resource overlay

This document describes every deviation from the standard `/add-resource` skill
that applies when implementing the `Invite` managed resource. It lives in the
`organization` group, so sections 1 and 2 of `docs/overlays/workspace.md`
(separate API group, Admin-key `ProviderConfig`) apply as well.

---

## 1. Create-only: no update endpoint

**Standard:** `ToAnthropicUpdate` plus an `Update()` that calls the SDK.

**Invite:** `BetaOrganizationInviteService` has `New`, `Get`, `List`, and
`Delete` only. There is no `ToAnthropicUpdate`; `Update()` is a no-op and
`Observe` reports `ResourceUpToDate: true` unconditionally, the same shape as
`Dream`. Every spec field (`email`, `role`, `rbacGroupIds`) is immutable;
changing one requires delete and recreate, which sends a new email.

---

## 2. Parameterless Get and Delete, no deletion policy

`Get(ctx, inviteID)` and `Delete(ctx, inviteID)` take no params struct. The
service has `Delete` only, so there is no `AnthropicDeletionPolicy` field.

---

## 3. Status drives existence

`Observe` treats an invite whose observed `status` is `deleted` (revoked out of
band) as absent, so Crossplane re-issues it. `pending`, `accepted`, and
`expired` invites stay observed and Ready; an expired invite is not re-sent
automatically.

`Delete` can only revoke a `pending` invite. On any error other than 404 it
re-reads the invite and treats a non-pending status (accepted, expired,
already revoked) as nothing left to delete.

---

## 4. E2E sends a real email

The example uses `email: ${data.anthropic_invite_email}`; the address comes from
the uptest datasource (`uptest-data.ini` locally, the `UPTEST_DATASOURCE`
secret in CI) so a throwaway inbox receives the invite, which the delete phase
revokes.

---

## Checklist for implementers

- [ ] `apis/organization/v1beta1/invite_types.go` — `Email`, `Role` (5-value enum), `RBACGroupIDs`; observation with `status`, `invitedAt`, `expiresAt`, `acceptedAt`
- [ ] `apis/organization/v1beta1/invite_conversion.go` — `ToAnthropicNew` and `FromAnthropicObservation` only
- [ ] `internal/controller/invite/reconciler.go` — no-op `Update`, `deleted` status as absent, non-pending tolerated in `Delete`
- [ ] `internal/controller/setup.go` — `invite.SetupGated(mgr, o)` (no bool arg)

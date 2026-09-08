# FederationRule — add-resource overlay

This document describes every deviation from the standard `/add-resource` skill
that applies when implementing the `FederationRule` managed resource. It lives
in the `organization` group and needs the WorkloadIdentityFederation identity
like `ServiceAccount` (see `docs/overlays/serviceaccount.md`, sections 1
and 2).

---

## 1. Three cross-resource references

`issuerId` (to `FederationIssuer`), `serviceAccountId` (to `ServiceAccount`,
sent as the rule's `target`), and `workspaceId` (to `Workspace`) all use the
standard `Ref`/`Selector` plumbing with `ComputedFieldExtractor("id")`. The
target is flattened: the API's `target: {type: service_account,
service_account_id}` becomes the single `serviceAccountId` field, and the
observation adds `serviceAccountName`.

---

## 2. Scope and match constraints the API enforces

Rules created through the API may grant `workspace:developer` or
`workspace:inference`; `org:admin` needs a Console session, so the
bootstrap rule that lets the provider itself mint org:admin tokens is created
by hand. `match` needs at least one of `subjectPrefix`, `claims`, or
`condition`; `audience` alone is rejected. Either `workspaceId` or
`appliesToAllWorkspaces: true` is required.

---

## 3. `attributes` is not modeled

The API documents `attributes` as not yet supported and rejects any non-empty
value with 400, so the CRD omits it.

---

## 4. Workspace-binding sub-resources are out of scope

`FederationRuleWorkspace`, `ServiceAccountWorkspace`, and
`WorkspaceServiceAccount` (add/list/remove bindings) are not modeled; a rule
binds one workspace through `workspaceId` or all of them through
`appliesToAllWorkspaces`.

---

## Checklist for implementers

- [ ] `apis/organization/v1beta1/federationrule_types.go` — `FederationRuleMatch`, three references, slug pattern on `name`
- [ ] `apis/organization/v1beta1/federationrule_conversion.go` — `target` from `serviceAccountId`, `matchParam`
- [ ] `internal/controller/federationrule/reconciler.go` — `Beta.Organization.Federation.Rules.*`, archived observed as absent
- [ ] `examples/organization/v1beta1/federationrule.yaml` — companion issuer and service account selected by label, `appliesToAllWorkspaces: true`

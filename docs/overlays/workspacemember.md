# WorkspaceMember — add-resource overlay

This document describes every deviation from the standard `/add-resource` skill
that applies when implementing the `WorkspaceMember` managed resource. It is a
sub-resource of `Workspace` and lives in the same `organization` group, so
sections 1 and 2 of `docs/overlays/workspace.md` apply as well.

---

## 1. Sub-resource of `Workspace` with a positional parent ID

**Standard:** one CRD maps to one flat service.

**WorkspaceMember:** the CRD maps onto
`client.Beta.Organization.Workspaces.Members`, reached under the parent
workspace service. The parent ID travels inconsistently, as in
`MemoryStoreMemory`:

| CRD operation | SDK call |
|---|---|
| Create | `Members.Add(ctx, workspaceID, AddParams{UserID, WorkspaceRole})` |
| Observe | `Members.Get(ctx, userID, GetParams{WorkspaceID})` |
| Update | `Members.Update(ctx, userID, UpdateParams{WorkspaceID, WorkspaceRole})` |
| Delete | `Members.Remove(ctx, userID, RemoveParams{WorkspaceID})` |

`ToAnthropicNew` therefore omits the workspace ID (positional) while
`ToAnthropicUpdate` sets `params.WorkspaceID`.

---

## 2. No identifier of its own: the external name is the user ID

**Standard:** the external-name annotation and `status.atProvider.id` hold an
Anthropic-assigned ID.

**WorkspaceMember:** a membership is addressed by the pair
(workspace ID, user ID) and the API returns no member ID. The reconciler
stores the **user ID** in the external-name annotation after `Add`, and the
observation has no `id` field, only `workspaceId`, `userId`, and
`workspaceRole`. Importing an existing membership means setting the
external-name annotation to the `user_...` ID.

---

## 3. Parent ID is a required cross-resource reference with reconciler guards

`workspaceId` uses the standard reference plumbing (`WorkspaceIDRef` /
`WorkspaceIDSelector` with `ComputedFieldExtractor("id")` pointing at
`Workspace`), and every reconciler method nil-checks it:

- **Observe:** unresolved → `ResourceExists: false`.
- **Create / Update:** unresolved → `errMissingWorkspace`.
- **Delete:** unresolved → no-op success.

---

## 4. Observe and Delete tolerate an archived or missing parent

Crossplane deletes a `Workspace` and its `WorkspaceMember`s concurrently, and
once the workspace is archived the members endpoint answers **400**, not 404
(`Cannot get a membership of an archived Workspace.`). Because the managed
reconciler runs `Observe` before `Delete`, a plain error there would leave the
member stuck with its finalizer. On any error other than 404, both `Observe`
and `Delete` fetch the workspace: archived or gone means the membership is
reported absent (Observe) or removed (Delete), while an active workspace
surfaces the original error.
`internal/controller/workspacemember/reconciler_test.go` covers every outcome
with a middleware-stubbed SDK client; the E2E run of 2026-09-08 hit the
Observe case for real.

---

## 5. Role enum excludes `workspace_billing`

`Add` rejects `workspace_billing` (it is inherited from the organization
billing role), so the spec enum lists the four assignable roles. The observed
`workspaceRole` has no enum because inherited billing members report it.
Organization admins and billing members are implicit members of every
workspace and can be neither added nor removed; E2E must use a user with the
`user` or `developer` organization role.

---

## 6. No `Metadata` field, no default-metadata wiring

`WorkspaceMemberParameters` has no map field, so `Setup`/`SetupGated` take no
`skipDefaultMetadata` argument and `setup.go` calls
`workspacemember.SetupGated(mgr, o)`.

---

## 7. E2E injects the user ID from the uptest datasource

The example uses `userId: ${data.anthropic_user_id}`. The value must be a
member with the `user` or `developer` organization role (admins and billing
members are implicit members of every workspace and cannot be added). CI
writes the whole datasource YAML from the `UPTEST_DATASOURCE` repository
secret into `.work/uptest-datasource.yaml` and sets `UPTEST_DATASOURCE_PATH`;
locally, write the same file and export the variable before `make uptest`.

---

## Checklist for implementers

- [ ] `apis/organization/v1beta1/workspacemember_types.go` — `WorkspaceID` + `Ref`/`Selector`, `UserID`, `WorkspaceRole` (4-value enum), observation without `id`
- [ ] `apis/organization/v1beta1/workspacemember_conversion.go` — `ToAnthropicNew` omits the parent ID, `ToAnthropicUpdate` sets it
- [ ] `internal/controller/workspacemember/reconciler.go` — user ID as external name, parent guards, archived-parent tolerant `Delete`
- [ ] `internal/controller/setup.go` — `workspacemember.SetupGated(mgr, o)` (no bool arg)
- [ ] `examples/organization/v1beta1/workspacemember.yaml` — `${data.anthropic_user_id}` placeholder and a companion `Workspace`

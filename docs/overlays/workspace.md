# Workspace — add-resource overlay

This document describes every deviation from the standard `/add-resource` skill
that applies when implementing the `Workspace` managed resource, the first
resource of the Admin API organization family.

---

## 1. Separate API group and package

**Standard:** every managed resource lives in
`apis/managedagents/v1beta1` under `managedagents.anthropic.crossplane.io`.

**Workspace:** organization resources live in `apis/organization/v1beta1`
under **`organization.anthropic.crossplane.io/v1beta1`**, mirroring the SDK's
`client.Beta.Organization` namespace. The group has its own
`groupversion_info.go` (registered in `apis/register.go`), its own CRD file
prefix (`organization.anthropic.crossplane.io_*.yaml`), and its own examples
directory (`examples/organization/v1beta1/`). Substitute
`apis/organization/v1beta1` wherever the standard steps say
`apis/managedagents/v1beta1`, and import it as `v1beta1` in the reconciler.

---

## 2. Admin API key ProviderConfig

**Standard:** examples rely on the `default` ClusterProviderConfig that
`cluster/test/setup.sh` creates from `UPTEST_CLOUD_CREDENTIALS`.

**Workspace:** every organization endpoint rejects regular API keys and needs
an Admin API key (`sk-ant-admin...`), which in turn cannot call the Messages or
Managed Agents APIs. Examples therefore set
`spec.providerConfigRef: {kind: ClusterProviderConfig, name: admin}`; setup.sh
creates that config from `UPTEST_ADMIN_CREDENTIALS`, and the E2E workflow
passes the `UPTEST_ADMIN_CREDENTIALS` repository secret. The identity type is
the unchanged `APIKey`.

---

## 3. SDK service path and parameterless Get/Archive

**Standard:** `client.Beta.<Resource>s.Get(ctx, id, Beta<Resource>GetParams{})`
and `Archive(ctx, id, Beta<Resource>ArchiveParams{})`.

**Workspace:** the service is `client.Beta.Organization.Workspaces`; `Get` and
`Archive` take the ID only, with no params struct. The SDK file is
`betaorganizationworkspace.go`, the params are
`BetaOrganizationWorkspaceNewParams` / `BetaOrganizationWorkspaceUpdateParams`,
and the response type is `BetaWorkspace`.

---

## 4. Archive only, and archiving is permanent

The service has `Archive` and no `Delete`, so there is no
`AnthropicDeletionPolicy` field. The Anthropic API cannot un-archive a
workspace and archives every API key created for it. `Observe` treats an
archived workspace as absent, so Crossplane re-creates one that was archived
out of band (the same convention as `Vault`).

---

## 5. Tags carry the default metadata

**Standard:** the default-metadata initializer writes the canonical Crossplane
identifiers into `spec.forProvider.metadata`.

**Workspace:** the API has no `metadata` field; its user-defined
`tags` map plays that role, so `Setup` wires
`initializer.New(mgr.GetClient(), "tags")`. Tag keys may not begin with
`anthropic`; the `crossplane-*` keys are fine.

---

## 6. Data residency is a nested object with a string-or-list union

`data_residency.allowed_inference_geos` is either the string `"unrestricted"`
or a list of geos. The CRD models it as `allowedInferenceGeos []string`, where
the single entry `unrestricted` selects the union's constant variant
(`constant.ValueOf[constant.Unrestricted]()`) and any other list maps to
`OfGeos`. `FromAnthropicObservation` maps back the same way, so the nested
object diffs key by key in `isUpToDate`. `workspaceGeo` is create-only and is
never sent on update.

---

## 7. `externalKeyId` is write-once

`ToAnthropicUpdate` sends `external_key_id` only when it differs from
`status.atProvider.externalKeyId`, because the API rejects any attempt to
replace an attached CMEK key. `FromAnthropicObservation` leaves the observed
field nil when the API returns an empty string.

---

## Checklist for implementers

- [ ] `apis/organization/v1beta1/workspace_types.go` — `Tags` (not `Metadata`), `DataResidency *WorkspaceDataResidency`, no `AnthropicDeletionPolicy`
- [ ] `apis/organization/v1beta1/workspace_conversion.go` — union mapping, write-once `ExternalKeyID`, `ArchivedAt` omitted
- [ ] `internal/controller/workspace/reconciler.go` — `Beta.Organization.Workspaces.*`, parameterless `Get`/`Archive`, initializer on `tags`
- [ ] `internal/controller/setup.go` — `workspace.SetupGated(mgr, o, skipDefaultMetadata)`
- [ ] `examples/organization/v1beta1/workspace.yaml` — `providerConfigRef` to the `admin` ClusterProviderConfig

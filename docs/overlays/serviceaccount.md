# ServiceAccount — add-resource overlay

This document describes every deviation from the standard `/add-resource` skill
that applies when implementing the `ServiceAccount` managed resource. It lives
in the `organization` group, so section 1 of `docs/overlays/workspace.md`
(separate API group) applies as well.

---

## 1. Needs the WorkloadIdentityFederation identity, not an API key

The service-account endpoints reject Admin API keys:
`403 permission_error: This endpoint requires an OAuth access token with the
org:admin scope`. Examples reference the `admin-federation`
`ClusterProviderConfig`, which `cluster/test/setup.sh` creates from
`UPTEST_FEDERATION_ORGANIZATION_ID` / `UPTEST_FEDERATION_RULE_ID` after
patching a projected ServiceAccount token into the provider's
`DeploymentRuntimeConfig`. The organization must trust the cluster's issuer
through an org:admin rule created once in the Console; the Kind cluster's
persistent signing key (`cluster/local/pki/`, restored in CI from the
`KIND_SA_KEY` secret) keeps that trust valid across clusters. See the README
section on workload identity federation for the bootstrap.

---

## 2. Standard Archive-only pattern with params structs

`Get(ctx, id, BetaOrganizationServiceAccountGetParams{})` and
`Archive(ctx, id, BetaOrganizationServiceAccountArchiveParams{})` take params
structs (unlike `Workspace`). No `Delete`, so no `AnthropicDeletionPolicy`.
`name` is immutable (absent from the update params) and only `description`
and `organizationRole` are sent on update.

---

## 3. Role constraints the API enforces

Creating an `admin`-role service account requires an interactive credential;
a workload (the provider itself, minting through a federation rule) can only
create `developer`-role accounts. A rule may grant `org:admin` only when its
target is an `admin`-role account.

---

## 4. Kind name shadows the core ServiceAccount in kubectl

`kubectl get serviceaccount` resolves to the core kind; address this one as
`serviceaccounts.organization.anthropic.crossplane.io` or by the `svac`
short name.

---

## Checklist for implementers

- [ ] `apis/organization/v1beta1/serviceaccount_types.go` — `Name`, `Description`, `OrganizationRole` (enum developer;admin)
- [ ] `apis/organization/v1beta1/serviceaccount_conversion.go` — `Name` omitted from update; shared `optionalString`/`optionalTime` helpers
- [ ] `internal/controller/serviceaccount/reconciler.go` — `Beta.Organization.ServiceAccounts.*`, archived observed as absent
- [ ] `examples/organization/v1beta1/serviceaccount.yaml` — `providerConfigRef` to `admin-federation`, random name

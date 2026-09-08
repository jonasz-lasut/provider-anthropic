# FederationIssuer — add-resource overlay

This document describes every deviation from the standard `/add-resource` skill
that applies when implementing the `FederationIssuer` managed resource. It
lives in the `organization` group and needs the WorkloadIdentityFederation
identity like `ServiceAccount` (see `docs/overlays/serviceaccount.md`,
sections 1 and 2).

---

## 1. `jwks` is a three-variant union flattened into one struct

`jwks` is `discovery` (OIDC discovery from the issuer URL, the API default),
`explicit_url`, or `inline`. The CRD flattens them into `FederationIssuerJWKS`
with a `type` discriminator; the conversion switches on `type` to fill
`OfDiscovery`, `OfExplicitURL`, or `OfInline`, and the flat response union is
copied back field by field.

---

## 2. Inline keys are arbitrary JSON

`jwks.keys` holds JWK objects as `[]apiextensionsv1.JSON` with
`x-kubernetes-preserve-unknown-fields`, because a JWK's shape depends on its
`kty`. The conversion unmarshals each entry into `map[string]any` for the SDK
and marshals the observed keys back; entries that are not JSON objects are
skipped. Drift on keys is compared structurally after both sides round-trip
through JSON, so formatting differences do not register.

---

## 3. Slug names are unique across archived issuers

`name` must be a slug and a duplicate returns 409, archived issuers included,
so the example uses `${Rand.RFC1123Subdomain}` for the name and issuer URL.
Inline keys make the example self-contained: Anthropic never fetches the
example's issuer URL.

---

## 4. Update-only field

`jwksPollingDisabled` exists only in the update params; the observation
exposes `jwksPollingDisabledAt` instead, so the two never diff against each
other.

---

## Checklist for implementers

- [ ] `apis/organization/v1beta1/federationissuer_types.go` — `FederationIssuerJWKS` union struct, `FederationIssuerPollStatus`, slug pattern on `name`
- [ ] `apis/organization/v1beta1/federationissuer_conversion.go` — variant switch, `jwksKeysToSDK` / `jwksKeysFromSDK`
- [ ] `internal/controller/federationissuer/reconciler.go` — `Beta.Organization.Federation.Issuers.*`, archived observed as absent
- [ ] `examples/organization/v1beta1/federationissuer.yaml` — inline JWKS, random slug

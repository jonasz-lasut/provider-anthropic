---
name: remediating-cves
description: Use when asked to remediate CVEs, security vulnerabilities, or GitHub code-scanning/Grype findings against a provider-anthropic release — e.g. "fix the CVEs from grype", "patch the vulnerabilities on the latest release", a security alert on a release tag, or a request to cut a security patch release.
---

# Remediating CVEs

## Overview

`.github/workflows/grype-scan.yaml` runs weekly, scans the ghcr image of the
**latest GitHub release** with Grype, and uploads the SARIF results to GitHub
code scanning against `refs/tags/<that release's tag>`. Remediating a CVE
found this way means patching the released version's branch and shipping a
new patch release end to end: branch → fix → tag → release → publish → sign.
That release only gets cut once remediation is actually complete — see the
completeness gate in step 4.

## When to use

- Asked to remediate/fix CVEs, vulnerabilities, GHSA advisories, or grype /
  code-scanning findings tied to a release of this provider.
- **Not** for routine dependency bumps with no security alert behind them
  (that's ordinary `chore(deps)` / Renovate / Dependabot work), and not for
  an Anthropic-SDK-specific bump with no CVE (use `/update-anthropic-sdk`).

## Procedure

### 1. Identify the target release branch

```bash
TAG=$(gh release view --json tagName -q .tagName)               # e.g. v1.0.0
BRANCH=release-$(echo "$TAG" | sed -E 's/^v([0-9]+)\.([0-9]+)\..*/\1.\2/')  # release-1.0
git fetch origin
git checkout "$BRANCH" 2>/dev/null || git checkout -b "$BRANCH" "$TAG"
```

If `origin/$BRANCH` doesn't exist yet, it must be cut from the release **tag**,
never from `main` HEAD — a security patch must not carry unreleased changes.

### 2. Pull the CVE list from code scanning

Pass `ref` as an actual **query parameter**, not a client-side filter. The
list endpoint only returns alerts for the default branch unless `ref` is
given in the request itself — fetching all alerts and filtering afterward
with `select(.most_recent_instance.ref==...)` silently returns an empty
result even when open alerts exist for the tag:

```bash
gh api "repos/{owner}/{repo}/code-scanning/alerts?ref=refs/tags/${TAG}&state=open" \
  --paginate \
  --jq ".[] | {number, rule: .rule.id, severity: .rule.security_severity_level, desc: .rule.description}"
```

For full detail (affected package, fixed-in version) on one alert:
`gh api repos/{owner}/{repo}/code-scanning/alerts/{alert_number}`. Triage
critical/high first. Keep this full list handy — step 4 checks it against
what actually got fixed.

### 3. Remediate on the release branch

**Exception — never bump `github.com/anthropics/anthropic-sdk-go` here.** If
a CVE's only fix is a version bump of the SDK itself, do not `go get` it as
part of this flow. Leave that CVE unremediated, keep going with everything
else, and take it to the completeness gate in step 4. The SDK has its own
dedicated sync workflow, `/update-anthropic-sdk`, because bumping it
requires syncing every resource's conversion/reconciler code to the new API
— a bare `go get` here risks silently breaking resource logic, which a
security patch branch has no business gambling on.

For everything else, Grype scans the built ghcr image, so the fix is one of:

- **Go module CVE** (any module other than the Anthropic SDK) →
  `go get <module>@<fixed-version> && go mod tidy`.
- **Go stdlib/toolchain CVE** → bump the `go` directive in `go.mod`.
- **Base image CVE** (`gcr.io/distroless/static-debian13:nonroot` in the
  Dockerfile) → bump the pinned base image tag/digest.

Then `make generate` (only if `apis/` types changed — unlikely for a pure dep
bump) and `make reviewable`. Commit locally as `fix(security): ...`, naming
the CVE/GHSA IDs remediated (and, if any were skipped under the exception
above, note that in the commit body too). Do not push yet — that's gated by
step 4.

### 4. Completeness gate — halt here if remediation is partial

Compare what got fixed against the full CVE list from step 2.

- **Every open CVE was remediated** → show the user the diff and the CVE
  list addressed, confirm before pushing, then continue to step 5. From here
  on, everything is externally visible (pushed branch, public tag, GitHub
  release, published package) and isn't something you can quietly undo by
  editing further.
- **One or more CVEs were skipped under the SDK exception in step 3** →
  **stop here.** Do not push, and do not trigger `Tag`, `Publish Provider
  Package`, or `Supply Chain and Xpkg Extensions` — none of steps 5–9 run.
  Leave the local commit(s) in place on `$BRANCH`, unpushed. Report:

  ```
  WARNING - provider update required!
  <CVE/GHSA ID> requires bumping anthropic-sdk-go to <fixed version>.
  Not applied by this flow. Run /update-anthropic-sdk to remediate, then
  re-run CVE remediation so the release covers every finding at once.
  ```

  One line per skipped CVE if there's more than one. The point of halting
  before push is that cutting a release now just means cutting a second one
  right behind it once the SDK bump lands — wait and ship one complete
  patch release instead.

### 5. Push

```bash
git push origin "$BRANCH"
```

### 6. Cut the patch tag — `Tag` workflow, from the release branch

```bash
NEW_VERSION=$(echo "$TAG" | awk -F. -v OFS=. '{$NF+=1} 1')   # v1.0.0 -> v1.0.1
gh workflow run Tag --ref "$BRANCH" -f version="$NEW_VERSION" \
  -f message="Security patch: <CVE/GHSA summary>"
```

### 7. Create the GitHub release

```bash
gh release create "$NEW_VERSION" --target "$BRANCH" \
  --title "$NEW_VERSION" \
  --notes "Security patch release. Remediates: <CVE/GHSA list>."
```

(`--target` only matters if the tag doesn't already exist; the `Tag` workflow
already created it, so this is just documentation-by-flag.)

### 8. Publish the package — `Publish Provider Package`, from the new tag

```bash
gh workflow run "Publish Provider Package" --ref "$NEW_VERSION" -f version="$NEW_VERSION"
```

Must run from the **tag**, not the branch — it builds the provider binary
from that exact tagged source.

### 9. Sign & attest — `Supply Chain and Xpkg Extensions`, from `main`

```bash
gh workflow run "Supply Chain and Xpkg Extensions" --ref main -f version="$NEW_VERSION"
```

Runs from `main`, not the tag — unlike step 8, this workflow doesn't build
anything from the ref it runs on. It signs/attests the image already
published to ghcr/xpkg.upbound.io by tag and appends Marketplace extensions
(SBOM, README, release notes), so it should use the newest signing logic on
`main` rather than whatever was frozen on the release branch at cut time.

### 10. Sequencing, verification, and reporting

Each of steps 6–9 must finish successfully before the next starts — step 8
needs the tag from step 6 to exist, step 9 needs the image step 8 published.
Find each run with `gh run list --workflow="<name>" --limit 1` and wait on it
(`gh run watch <run-id> --exit-status`); if using the Monitor tool, its
until-loop pattern is the sanctioned way to poll rather than a manual sleep
loop. Once all three conclude, optionally trigger `Grype Vulnerability Scan`
manually to confirm the new tag's image is clean, then report the CVE/GHSA
IDs remediated and the new version released.

## Quick reference

| Workflow | Ref to run from | Key inputs |
|---|---|---|
| `Tag` | release branch (`release-X.Y`) | `version`, `message` |
| (n/a) `gh release create` | — | tag name, `--target` branch |
| `Publish Provider Package` | the new tag (`vX.Y.Z+1`) | `version` |
| `Supply Chain and Xpkg Extensions` | `main` | `version` |

## Common mistakes

- Pushing or triggering any workflow when a CVE was skipped under the SDK
  exception — the run must halt at step 4 instead. A partial security
  release just buys a second release right behind it once the SDK bump
  lands; wait and ship one complete patch release.
- Bumping `anthropic-sdk-go` directly to close a CVE instead of flagging it —
  it has its own sync workflow for a reason; a quick `go get` here can leave
  reconciler/conversion code silently out of sync with the new API.
- Cutting `release-X.Y` from `main` HEAD instead of the release tag — leaks
  unreleased changes into a security patch.
- Running `Publish Provider Package` against the branch instead of the new
  tag — builds a moving target if the branch gets more commits later.
- Running `Supply Chain and Xpkg Extensions` from the tag instead of `main`
  — it still works, but forfeits any signing/attestation fixes landed on
  `main` since the release branch was cut.
- Triggering step 8 or 9 before the prior workflow run has actually finished
  — the tag or image it depends on won't exist yet.
- Fetching `code-scanning/alerts` with no `ref` query parameter and filtering
  by `most_recent_instance.ref` afterward — this returns an empty list even
  when the tag has real open alerts, reading as "no CVEs found" instead of
  a query bug. Always pass `ref=refs/tags/<tag>` in the request itself.

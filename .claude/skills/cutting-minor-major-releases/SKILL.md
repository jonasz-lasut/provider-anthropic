---
name: cutting-minor-major-releases
description: Use when asked to cut a new minor or major release of provider-anthropic, ship a batch of merged main changes as a new version, or bump to a specific vX.Y.0 / vX.0.0 — e.g. "cut a minor release", "ship v1.1.0", "release the next major version", "bump to v2.0.0". Not for patch releases remediating CVEs (see remediating-cves) or ad hoc patch bumps.
---

# Cutting a Minor or Major Release

## Overview

Cutting a minor or major release ships everything currently merged to `main` as
a brand-new version line: a new `release-X.Y` branch, tag, GitHub release,
published package, and signed/attested artifacts. Unlike a CVE security patch
(see `remediating-cves`), there is no fix to author — the branch is cut
straight from `main` HEAD, and the version bump (minor vs major) is chosen by
the user, never inferred.

## When to use

- Asked to cut/ship a new minor or major release, "release vX.Y.0", "bump to
  vX.0.0", or ship a batch of merged features as a new version.
- **Not** for patch releases remediating CVEs/security findings on an
  already-released version — that reuses an *existing* release branch cut
  from a tag (see `remediating-cves`).
- **Not** for ad hoc patch-level bumps (`vX.Y.Z+1`) with no security finding
  behind them — this skill only computes minor or major bumps.
- The bump type (minor vs major) must come from explicit user input. Never
  infer it from the size of the diff — ask if it isn't stated.

## Procedure

### 1. Determine current version and confirm bump type

```bash
TAG=$(gh release view --json tagName -q .tagName)   # e.g. v1.0.0
MAJOR=$(echo "$TAG" | sed -E 's/^v([0-9]+)\.([0-9]+)\..*/\1/')
MINOR=$(echo "$TAG" | sed -E 's/^v([0-9]+)\.([0-9]+)\..*/\2/')
```

If the user has already stated minor or major (e.g. "cut a minor release"),
use that directly. Otherwise ask before computing anything. Then:

- **major** → `NEW_VERSION="v$((MAJOR+1)).0.0"`
- **minor** → `NEW_VERSION="v${MAJOR}.$((MINOR+1)).0"`

```bash
NEW_BRANCH="release-$(echo "$NEW_VERSION" | sed -E 's/^v([0-9]+)\.([0-9]+)\..*/\1.\2/')"
```

### 2. Verify `main` is release-ready

```bash
git fetch origin
git checkout main && git pull
gh run list --branch main --limit 1     # confirm the latest CI run on main is green
```

There is no remediation step here — whatever is merged to `main` is what
ships, and unlike CVE remediation there is no fix step or completeness gate
downstream to catch a bad `main` later. If the latest run's conclusion isn't
`success`, **stop and report** — do not cut the branch until `main` is green.
CI on `main` already covers `make reviewable`/`check-diff`, so a green run is
a legitimate signal.

### 3. Cut the new release branch from `main` HEAD

```bash
if git ls-remote --exit-code --heads origin "$NEW_BRANCH"; then
  echo "ALREADY EXISTS - abort: this version line was already released"
else
  git checkout -b "$NEW_BRANCH" main
fi
```

If the branch already exists, **stop and report** rather than continuing —
do not fall through to checkout/push.

This is the mirror image of the CVE flow: a security patch reuses an
*existing* `release-X.Y` cut from a tag; a new minor/major line is always a
**brand-new** branch cut from `main` HEAD. If `origin/$NEW_BRANCH` already
exists, this version line was already released — stop and report instead of
pushing over it.

### 4. Confirm before anything externally visible

Show the user `NEW_VERSION`, `NEW_BRANCH`, and a summary of what's shipping
(e.g. `git log --oneline "$TAG"..main`). From here on every step is
externally visible (pushed branch, public tag, GitHub release, published
package) and isn't something you can quietly undo — get explicit
confirmation before continuing.

### 5. Push

```bash
git push -u origin "$NEW_BRANCH"
```

### 6. Cut the tag — `Tag` workflow, from the new release branch

```bash
gh workflow run Tag --ref "$NEW_BRANCH" -f version="$NEW_VERSION" \
  -f message="Release $NEW_VERSION"
```

### 7. Create the GitHub release

```bash
gh release create "$NEW_VERSION" --target "$NEW_BRANCH" \
  --title "$NEW_VERSION" \
  --notes "<summary of what's new since $TAG>"
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

Runs from `main`, not the tag — it signs/attests the image already published
to ghcr/xpkg.upbound.io by tag and appends Marketplace extensions (SBOM,
README, release notes), so it should use the newest signing logic on `main`
rather than whatever was frozen on the release branch at cut time.

### 10. Sequencing, verification, and reporting

Each of steps 6–9 must finish successfully before the next starts — step 8
needs the tag from step 6 to exist, step 9 needs the image step 8 published.
Find each run with `gh run list --workflow="<name>" --limit 1` and wait on it
(`gh run watch <run-id> --exit-status`); if using the Monitor tool, its
until-loop pattern is the sanctioned way to poll rather than a manual sleep
loop. Once all three conclude, report the new version released and the
branch it lives on.

## Quick reference

| Step | Ref to run from | Key inputs |
|---|---|---|
| Cut branch | `main` HEAD | new `release-X.Y` |
| `Tag` | new release branch (`release-X.Y`) | `version`, `message` |
| (n/a) `gh release create` | — | tag name, `--target` branch |
| `Publish Provider Package` | the new tag (`vX.Y.0` / `vX.0.0`) | `version` |
| `Supply Chain and Xpkg Extensions` | `main` | `version` |

## Common mistakes

- Cutting the release branch from an existing tag instead of `main` HEAD —
  that's the CVE-patch pattern; a new minor/major line must include
  everything merged to `main`.
- Inferring minor vs major from the diff instead of the user's explicit
  input.
- Not checking whether `NEW_BRANCH` already exists — each `release-X.Y` line
  is cut exactly once; if it exists, this version was already released.
- Cutting from a red `main` — unlike CVE remediation there's no fix step or
  completeness gate to catch this, so a broken `main` ships broken as-is.
- Running `Publish Provider Package` against the branch instead of the new
  tag — builds a moving target if the branch gets more commits later.
- Running `Supply Chain and Xpkg Extensions` from the tag instead of `main`
  — it still works, but forfeits any signing/attestation fixes landed on
  `main` since the release branch was cut.
- Triggering step 8 or 9 before the prior workflow run has actually finished
  — the tag or image it depends on won't exist yet.
- Using this skill for a patch bump (`vX.Y.Z+1`) — that path belongs to
  `remediating-cves`, which reuses an existing branch instead of cutting one.

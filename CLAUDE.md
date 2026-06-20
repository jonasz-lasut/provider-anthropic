# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`provider-anthropic` is a [Crossplane](https://crossplane.io/) v2 provider that manages
objects on the Anthropic platform's [Managed Agents beta API](https://docs.anthropic.com/en/api/managed-agents-overview)
(Agents, Sessions, Vaults, Skills, Memory stores, …) as Kubernetes managed resources. It is a
hand-written provider built directly on the `anthropic-sdk-go` — **not** an Upjet/Terraform-based
provider, despite reusing the Crossplane build submodule and Uptest tooling.

## Common commands

The `build/` directory is a git submodule (`makelib`). On a fresh clone, bootstrap it first:

```console
make submodules          # required once after clone; targets fail until this runs
```

| Task | Command |
|---|---|
| Build the provider binary | `make build` (or just `make`) |
| Run unit tests | `make test` |
| Run a single test | `go test ./internal/controller/session/... -run TestObserve` |
| Lint | `make lint` (golangci-lint v2.12.1) |
| Regenerate CRDs + deepcopy + managed methodsets | `make generate` |
| Full pre-PR gate (generate + lint + test) | `make reviewable` |
| Verify generated files are committed | `make check-diff` |
| Run controller locally, out-of-cluster | `make run` (needs a reachable cluster — see README) |
| Build + deploy into a local Kind cluster | `make local-deploy` |
| Tear down the local cluster | `make controlplane.down` |

After **any** change to `apis/` types, run `make generate` and commit the regenerated
`zz_generated.*.go` files and `package/crds/*.yaml` — CI's `check-diff` fails otherwise.

## Architecture

### Layering

```
apis/<group>/<version>/        CRD Go types + handwritten conversion + generated zz_* files
  config/v1alpha1/             ProviderConfig / ClusterProviderConfig (credential plumbing)
  beta/v1alpha1/               every managed resource + ObservedAgentCollection + Predicates
internal/clients/              Anthropic SDK client builder, secret resolution, drift diffing
internal/controller/<kind>/    one package per resource: reconciler.go implements ExternalClient
internal/controller/setup.go   SetupProviders wires every controller (gated on CRD readiness)
cmd/provider/main.go           manager bootstrap, scheme registration, CRD gate
package/crds/                  generated CRD manifests (do not hand-edit)
examples-generated/            example manifests used as the E2E test corpus
```

`apis/generate.go` drives code generation via `//go:generate` (controller-gen for CRDs/deepcopy,
angryjet for crossplane-runtime methodsets, then `hack/stamp-sdk-version.sh` stamps the SDK
version into every CRD).

### The managed-resource pattern (read `internal/controller/agent/reconciler.go` as the canonical example)

Each resource follows the crossplane-runtime `managed.ExternalClient` contract with four moving parts:

1. **`Setup` / `SetupGated`** — `SetupGated` registers the controller with a CRD *gate* so it only
   starts once its CRD is established. `cmd/provider/main.go` runs `customresourcesgate` to flip
   these gates. All wiring goes through `SetupProviders` in `setup.go`.
2. **`connector.Connect`** — builds an authenticated `*anthropic.Client` via `clients.NewClient`
   (which resolves the `ProviderConfig`, tracks usage, and extracts the API key from the
   credentials). Credentials are a JSON payload (e.g. `{"api_key":"sk-ant-…"}`); `spec.identity.type`
   on the `ProviderConfig` selects how it's parsed — `APIKey` is currently the only identity type
   (`internal/clients/anthropic.go` → `apiKeyFromCredentials`).
3. **`external` (Observe/Create/Update/Delete)** — translates between the CRD and the SDK using the
   conversion methods on the API type (`ToAnthropicNew`, `ToAnthropicUpdate`, `FromAnthropicObservation`).
4. **`isUpToDate`** — drift detection (see below).

**External-name handling is subtle:** Crossplane seeds the external-name annotation with the
Kubernetes object name *before* `Create` runs. Some Anthropic endpoints return 400 (not 404) for
non-prefixed IDs, so "not yet created" is detected by comparing the external name against
`mg.GetName()`, not by checking for empty. After `Create`, the real ID (`agent_…`, `skl_…`, …) is
stored in both the external-name annotation and `Status.AtProvider.ID`.

### Conversion layer (`apis/beta/v1alpha1/*_conversion.go`)

Conversion lives on the API types, not in the reconciler, and is unit-tested per resource
(`*_conversion_test.go`). Resources with secret-valued fields define a `<Resource>ConversionContext`
that carries resolved secret strings and publishes them as Crossplane connection details via
`ToConnectionDetails()`.

### Drift detection (`internal/clients/diff.go`)

`isUpToDate` JSON-marshals `Spec.ForProvider` and `Status.AtProvider` and calls
`clients.IsSubsetEqual`: every key in *desired* must equal *observed*, but keys absent from
observed are skipped (handles ForProvider-only fields like SecretRefs, refs/selectors). Special
case: a desired plain-string ID is compared against an observed `{id: …}` object. Secret-valued
fields (e.g. an Agent's `system` prompt) are stored as a SHA-256 in `AtProvider` and compared
separately, since the secret never appears in the API response.

### Resources that deviate from the standard pattern

Each deviating resource has a `docs/overlays/<lowercase-resource>.md` that lists every departure
from the standard `/add-resource` procedure. The skill's Step 0 reads the overlay before
scaffolding, so always read the overlay before touching the resource.

- **`Skill`** maps one CRD onto two SDK services (`Skills` + `Skills.Versions`), uploads files via
  multipart (not JSON), and holds a long-lived content-addressed filesystem cache
  (`internal/controller/skill/fs.go`, `internal/capabilities/fs.go`). `Update` creates a new
  version rather than patching. The full deviation list is in **`docs/overlays/skill.md`** — read it
  before touching anything under `skill/`.
- **`MemoryStoreMemory`** and **`VaultCredential`** are *sub-resources*: each maps onto a service
  nested under a parent (`Beta.MemoryStores.Memories`, `Beta.Vaults.Credentials`) whose methods take
  the parent ID as a **positional argument** on `New` and inside the params struct on
  Get/Update/Delete. The parent ID (`memoryStoreId` / `vaultId`) is a *required* cross-reference and
  every reconciler method guards on it. `VaultCredential` additionally maps a discriminated `Auth`
  union with five SecretRefs across its variants. See **`docs/overlays/memorystorememory.md`** and
  **`docs/overlays/vaultcredential.md`**.
- **`ObservedAgentCollection`** is *not* a managed resource. It lists agents from the API and
  materializes one Observe-only `Agent` child per match via Server-Side Apply, deleting stale
  children. It uses `NewClientFromProviderConfig` (no usage tracker) and filters results with
  `internal/predicates/` (`MetadataMatch`, `CELFilter` client-side; `CreatedAtGte/Lte` server-side).
  See the README "Filtering collections with predicates" section for the user-facing surface.

## Slash commands (`.claude/commands/`)

This repo ships authoring automations. Each embeds full step-by-step instructions and runs
`make generate` / `make local-deploy` as part of its flow — invoke the command rather than
reproducing the steps by hand.

| Command | When to use it |
|---|---|
| `/add-resource ResourceName[,…]` | Adding a brand-new managed resource backed by a `Beta<Resource>` SDK service. Scaffolds the types file, conversion, reconciler, controller wiring, and `setup.go` entry, then generates. |
| `/add-collection ResourceName[,…]` | Adding an `Observed<Resource>Collection` for a resource whose SDK service exposes `List`/`ListAutoPaging`. Wires the materializing reconciler + predicate plumbing. |
| `/generate-examples` | Regenerating `examples-generated/` manifests for every CRD (feasible defaults, used as the E2E corpus). |
| `/update-anthropic-sdk` | Bumping the `anthropic-sdk-go` dependency and syncing all reconciler/conversion logic to upstream API changes, then re-stamping the version into the CRDs. |

When asked to "add a resource", "support kind X", "add a collection", "regenerate examples", or
"update the SDK", reach for the matching command first — these encode this repo's exact conventions
(external-name handling, gated setup, conversion + drift patterns) that are easy to get subtly wrong.

## End-to-end testing the resources

E2E tests run the real provider against the **real Anthropic API** using
[Uptest](https://github.com/crossplane/uptest) over the manifests in `examples-generated/`.

```console
UPTEST_CLOUD_CREDENTIALS="YOUR_ANTHROPIC_API_KEY" \
UPTEST_EXAMPLE_LIST="examples-generated/beta/v1alpha1/agent.yaml" \
make e2e
```

- `make e2e` = `make local-deploy` (spin up Kind, build + install the provider, wait Healthy) then
  `make uptest`.
- `UPTEST_EXAMPLE_LIST` is a comma-separated list of example files to apply, wait for
  `Ready,Synced`, then delete.
- `cluster/test/setup.sh` runs before the suite: when `UPTEST_CLOUD_CREDENTIALS` is set it creates
  the `provider-secret` Secret and a `default` `ClusterProviderConfig` so examples reconcile.
- Each example manifest carries `testing.upbound.io/example-name` / `meta.upbound.io/example-id`
  labels that Uptest relies on — keep them when editing examples.

**In CI**, E2E is triggered by a maintainer commenting on a PR:
`/test-examples="examples-generated/beta/v1alpha1/agent.yaml"` (see `.github/workflows/e2e.yaml`).
It runs only for users with write/admin permission and reports back as a commit status check.

For changes that don't need the live API, prefer the per-resource conversion unit tests
(`apis/beta/v1alpha1/*_conversion_test.go`) and `internal/controller/session/reconciler_test.go`.

## Conventions

- **Generated files** (`zz_generated.*.go`, `package/crds/*.yaml`, `examples-generated/`) are never
  hand-edited — change the source types/conversion and run `make generate`.
- **API groups:** credential/config types live under `anthropic.crossplane.io` (`apis/config`);
  every managed resource lives under `beta.anthropic.crossplane.io/v1alpha1` (`apis/beta`).
- The provider declares the `SafeStart` capability; new controllers must register through
  `SetupGated` so they wait for their CRD.

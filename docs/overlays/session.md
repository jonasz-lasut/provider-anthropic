# Session — add-resource overlay

`Session` otherwise follows the standard `/add-resource` pattern. This overlay
records one deliberate **non-adoption** so a future `/update-anthropic-sdk` or
`/add-resource` re-run does not scaffold it automatically.

---

## Deferred: the `agent_with_overrides` agent variant

As of `anthropic-sdk-go` v1.55.0, `BetaSessionNewParamsAgentUnion` gained a third
variant, `agent_with_overrides` (`OfBetaManagedAgentsAgentWithOverridess`),
alongside the plain agent-ID string and the agent-ID+version object. It
references an existing agent and lets the caller override `system`, `version`,
`mcpServers`, `model`, `skills`, and `tools` **for that session only** — per the
SDK, "the agent resource is unchanged."

We intentionally expose only `agentId` (+ optional `agentVersion`) and do **not**
model `agent_with_overrides`, for three reasons:

1. **It inlines a second resource's template into Session.** The override block is
   essentially a full `Agent` spec (system prompt, model config, skills/tools
   unions, MCP servers) embedded inside `Session.spec.forProvider`. That
   duplicates the `Agent` CRD schema and blurs the one-CRD-per-API-object
   boundary the provider keeps elsewhere. It does **not** mutate the referenced
   Agent (so there is no shared-ownership/dual-write hazard on the Agent object),
   but it does make Session own configuration that conceptually belongs to an
   Agent.

2. **The overrides are create-only — they break continuous reconciliation.** A
   session's agent binding is fixed at creation; `ToAnthropicUpdate` only patches
   `title`/`metadata`. Override fields would therefore be immutable after create:
   editing the inlined system prompt or tool list in the spec could not be pushed
   to the API, producing silent drift (or forcing resource replacement) — an
   anti-pattern for a declarative managed resource.

3. **It reintroduces awkward secret handling with no update path.** The `system`
   override is a prompt string (the `Agent` CRD takes it as a write-only
   `SystemSecretRef` with SHA-256 drift hashing). Inlining it here would mean a
   write-only, immutable, hash-compared field on Session — complexity with no
   reconcile benefit.

**Preferred alternative:** point a Session at a real managed `Agent` via
`agentId`/`agentIdRef` (already supported) and let the `Agent` resource own its
configuration. If per-session variation is genuinely needed, create a distinct
managed `Agent` and reference it, rather than inlining an override template into
Session.

Revisit only if a concrete use case requires ephemeral, session-scoped agent
configuration that cannot be expressed as a separate `Agent` — and if so, design
it deliberately (likely as create-only fields with explicit immutability and
drift documentation), not as an automatic SDK-sync addition.

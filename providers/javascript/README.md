# JavaScript code-fact provider

`waldo-javascript-provider` owns generic JavaScript and TypeScript runtime semantics. Its current implementation uses
an embedded Semgrep rule set, but consumers configure the provider rather than Semgrep and never supply language-rule
files for built-in facts.

The provider currently recognizes three deliberately narrow source shapes:

- an assigned asynchronous timer callback;
- module-local object or map state written and read across calls; and
- module-local state used as a high-confidence cross-request coordination predicate.

The timer shape emits:

```text
kind: deferred-execution
execution.authority: process-local
execution.callback: async
execution.scheduler: timer
```

The provider emits `correctness.criticality: unknown`; JavaScript syntax establishes the scheduling authority, not
whether completion is an architectural requirement. The built-in warning can therefore surface the loss mode, while
the error-level `durable-deferred-execution` invariant still requires separate evidence that
`correctness.critical: true`.

The broad local-state shapes emit a `state-dependent-decision` fact with `state.scope: process-local` and
`state.role: unknown`. The provider does not guess whether a durable fallback makes the source safe. The
`process-local-authority` warning lets developers retain that evidence as accepted or false-positive. Providers with
stronger provenance may emit `state.role: authority` or `state.role: cache`; the latter does not match the policy.

The narrower state-handoff shape emits a high-confidence `coordination` fact only when the source establishes a
cross-request predicate with deployment-wide required scope. This supports the separate `process-local-coordination`
error without turning every local map into a coordination failure.

With a topology-only `waldo.yaml`, Waldo selects this provider automatically and derives targets from deployment
artifact source values. Repeated sources are scanned once. Conventional `*.test.*` and `*.spec.*` JavaScript and TypeScript
files are excluded by default.

An explicit provider entry is an advanced full override. For example:

```yaml
providers:
  - name: javascript
    command:
      - waldo-javascript-provider
      - --target
      - services/worker/src
      - --exclude
      - "**/*.test.ts"
```

Targets and exclusions select source owned by the deployed artifact; repeat either flag as needed. Exclusions are
provider configuration rather than backend configuration, so consumers do not depend on Semgrep's private rule
format.

Semgrep CE must currently be available to the provider process, but it is an implementation detail. The backend can
be replaced without changing emitted facts, deployment models, policies, or findings.

Repository CI installs a pinned Semgrep CE release and requires the source fixture test to execute. The scan uses
only the provider's embedded local rules, disables metrics and version checks, and needs no Semgrep account or token.
Local Go test runs may skip that integration test when Semgrep is unavailable; users still invoke `waldo check`, not
Semgrep directly.

The timer rule deliberately covers assigned inline async callbacks. Wrapper functions, separately declared
callbacks, unassigned timers, intervals, cron libraries, and criticality classification remain explicit false-negative
boundaries until real source examples justify broader provider support.

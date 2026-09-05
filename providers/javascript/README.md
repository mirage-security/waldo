# JavaScript code-fact provider

`waldo-javascript-provider` owns generic JavaScript and TypeScript runtime semantics. Its current implementation uses
an embedded Semgrep rule set, but consumers configure the provider rather than Semgrep and never supply language-rule
files for built-in facts.

The initial source-backed fact recognizes an assigned asynchronous `setTimeout` callback and emits:

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

With a topology-only `waldo.yaml`, Waldo selects this provider automatically and derives targets from deployment
`codeRoots`. Conventional `*.test.*` and `*.spec.*` JavaScript and TypeScript files are excluded by default.

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

Targets and exclusions select source owned by the deployed unit; repeat either flag as needed. Exclusions are
provider configuration rather than backend configuration, so consumers do not depend on Semgrep's private rule
format.

Semgrep CE must currently be available to the provider process, but it is an implementation detail. The backend can
be replaced without changing emitted facts, deployment models, policies, or findings.

Repository CI installs a pinned Semgrep CE release and requires the source fixture test to execute. The scan uses
only the provider's embedded local rules, disables metrics and version checks, and needs no Semgrep account or token.
Local Go test runs may skip that integration test when Semgrep is unavailable; users still invoke `waldo check`, not
Semgrep directly.

The first rule deliberately covers assigned inline async callbacks. Wrapper functions, separately declared
callbacks, unassigned timers, intervals, cron libraries, and criticality classification remain explicit false-negative
boundaries until real source examples justify broader provider support.

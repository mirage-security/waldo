# Waldo agent guide

## Project thesis

Waldo evaluates architectural invariants by combining facts about code with facts about how that code is deployed.

Protect this boundary in every change. Waldo must not become a collection of static-analysis rules that happen to be
useful. A core policy belongs here only when its conclusion requires both code facts and deployment facts.

## Current foundation

The source-backed policies currently claimed by the project are `durable-deferred-execution` and
`process-local-coordination`. The durable-execution proof uses:

- standalone source in `examples/durable-deferred-work/app/`;
- the separate provider in `providers/goast/`;
- the public process contract in `protocol/`;
- one shared policy in `policies/durable-deferred-execution.yaml`; and
- two models in `examples/durable-deferred-work/*.waldo.yaml`.

Both models must continue to load the same policy and produce the same stable unresolved-error identity. Preserve
`internal/foundation/TestOneInvariantAcrossTwoDeploymentModels` as the proof of that claim.

The coordination proof uses the separate Semgrep adapter, a narrow provider-side source rule, one shared policy, and
replicated/single-instance deployment models under `examples/process-local-coordination/`. Error severity requires a
high-confidence fact with deployment-wide required scope. `replica-local-authority` remains a warning; a local cache
under multiple replicas is not by itself an error. Do not add named lock, dedupe, uniqueness, or leadership variants
until a real source example requires a distinct semantic fact or message.

## Architectural ownership

- `cmd/waldo` and `internal/` own orchestration, configuration, policy joins, dispositions, reporting, and comparison.
  They must remain language- and analyzer-agnostic.
- `protocol/` owns the public versioned request and code-fact contract. External providers must be able to implement
  the JSON protocol without importing Waldo internals.
- `providers/` and provider commands own source syntax, runtime semantics, framework behavior, package discovery, and
  dataflow. Core must not import provider packages.
- `waldo.yaml` and other deployment models own objective deployment properties. They must not contain
  programming-language runtime semantics.
- `policies/` owns provider-neutral architectural invariants. Policy IDs, matches, messages, and severity are data,
  not hard-coded behavior in core.
- Technology-specific remediation belongs in integrations, not core findings or policies.
- Human decisions change finding disposition only. They never mutate facts, severity, or global policy behavior.

## Rule admission test

Before adding or changing a core policy, prove all of the following:

1. Without the code fact, the architectural conclusion does not follow.
2. Without the deployment fact, the architectural conclusion does not follow.
3. The invariant and finding message name no programming-language API or infrastructure product.
4. Concrete syntax is normalized by a provider into provider-neutral code facts.
5. Platform details are normalized into objective deployment facts.
6. The finding describes an architectural consequence rather than prescribing a product.
7. The severity follows from the strength of the joined evidence.

If a check still works after removing deployment context, put it in an analyzer instead of Waldo core. If a check
still works after removing code context, put it in deployment validation.

For example, stale state reused after an asynchronous yield is only a source-level observation. Keep that check in a
language/runtime analyzer unless a proposed core policy also requires explicit execution and deployment facts proving
that the suspension admits contenders which share the authority, plus an architectural requirement for atomicity. A
runtime that preserves exclusive ownership through the operation is a required negative case.

Every policy contribution needs this executable matrix:

| Code facts | Deployment facts | Expected result |
| --- | --- | --- |
| Match | Match | Finding |
| Match | Do not match | No finding |
| Do not match | Match | No finding |

Prefer a second positive deployment model demonstrating that the same policy survives a change in platform context.
Include accepted-disposition and known-noise scenarios when they are relevant.

## Severity

- Use `error` only for a direct contradiction established by the joined facts. Process-local exclusion under multiple
  independent instances is the model: it cannot provide fleet-wide exclusion.
- Use `warning` when intent, provenance, or dataflow can still make the observed pattern safe. Broad local-state
  authority remains warning-level.
- Use `info` for retained evidence that does not currently require review or fail CI.

CI fails only `severity: error` plus `disposition: unresolved`. Accepted and false-positive findings remain visible.

## Provider and identity requirements

- Keep provider processes replaceable through protocol v1 JSONL.
- Providers may recognize specific APIs, but core policies match normalized semantic attributes.
- Do not require heavyweight analysis for facts a small structural provider can establish.
- Fact IDs must be stable across line movement and must not use line numbers as identity.
- Finding locations are evidence, not identity.
- Do not add symbol-wide allowance matching as a general disposition system.

## Scope exclusions

Do not add:

- language parsers, API names, or direct source discovery to Waldo core;
- infrastructure product names or product recommendations to core policies;
- application-specific paths, deployment adapters, findings, or dispositions;
- a generalized assumption that all timers, caches, or local state are incorrect;
- hard-coded rule IDs or severities in evaluation code; or
- new policies without the cross-boundary evidence matrix.

Read `CONTRIBUTING.md`, `docs/architecture.md`, `docs/provider-protocol.md`, and `docs/policy-taxonomy.md` before changing
an architectural boundary or proposing a policy.

## Development workflow

The project uses Go 1.27.1. Format and verify changes with:

```sh
gofmt -w cmd internal protocol providers examples
go test ./...
go vet ./...
go build ./cmd/...
git diff --check
```

An intentional unresolved error makes `waldo check` exit `1`; that is the expected result for both foundation example
models. Configuration, provider, and input failures exit `2`.

Keep generated binaries and reports out of the repository. Preserve unrelated user changes and keep commits scoped to
the requested work.

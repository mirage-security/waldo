# Waldo

Waldo finds architectural mismatches by joining four independent inputs:

```text
deployment facts + code facts + architectural policy + human disposition -> findings
```

The core is language- and analyzer-agnostic. OpenGrep, GritQL, compiler-backed analyzers, and small structural scanners
can all act as code-fact providers through the same JSONL process protocol. JavaScript runtime semantics, language
parsing, and framework behavior belong in those providers—not in the deployment model or Waldo core.

## Status

This is an early standalone implementation. The public deployment-model file is `waldo.yaml`; its schema and the
provider protocol are both versioned at `1`.

## Quick start

Build the CLI:

```sh
go build -o bin/waldo ./cmd/waldo
```

After adding at least one provider, run it and evaluate its facts:

```sh
bin/waldo check --root . --config waldo.yaml
```

For a deterministic or precomputed scan, bypass configured providers with a JSONL facts file:

```sh
bin/waldo check --root . --config waldo.yaml --facts facts.jsonl --json
```

`check` exits `1` only when at least one finding is both `severity: error` and `disposition: unresolved`. Accepted and
false-positive findings stay in the report as evidence. Configuration, provider, input, and missing fact-source
failures exit `2`, so an accidentally unconfigured CI scan cannot silently pass.

Compare two check reports to isolate a proposed change from existing debt:

```sh
bin/waldo compare --base base.report.json --head head.report.json --json
```

`compare` exits `1` for a newly introduced unresolved error, or when a changed finding becomes an unresolved error.

## Configuration model

[`waldo.yaml`](waldo.yaml) is the provider-free root model. A deployment unit maps code paths to objective facts:

```yaml
deployment:
  units:
    worker:
      codeRoots: [services/worker]
      facts:
        process.restartable: true
        process.replicas: 3
        scheduling.processLocal.durable: false
```

Policies are data. Waldo core does not hard-code rule IDs or analyzer-specific syntax:

```yaml
policyFiles:
  - policies/durable-deferred-execution.yaml
```

Policy files are resolved relative to the `waldo.yaml` file. Inline `policies` remain supported for self-contained
models, while shared files let multiple deployment models prove the same invariant without copying or changing it.

Conditions accept exact scalar values and the operators `equals`, `notEquals`, `greaterThan`,
`greaterThanOrEqual`, `lessThan`, `lessThanOrEqual`, and `oneOf`.

Dispositions target a full stable finding identity printed by the first scan:

```yaml
dispositions:
  - finding: waldo:v1:012345...
    disposition: accepted
    reason: This use is deliberately best-effort and product behavior permits loss.
```

Only `accepted` and `false-positive` can be configured, and both require a reason. A resolved finding is represented by
its absence from a later report, not by a `fixed` disposition. Finding identities derive from policy ID, deployment
unit, provider name, and the provider's stable fact ID; file locations remain evidence and may move without changing
the identity.

See [the provider protocol](docs/provider-protocol.md) and [architecture notes](docs/architecture.md) for the boundary
contracts.

## Executable foundation proof

The [durable deferred-work example](examples/durable-deferred-work/README.md) analyzes standalone Go source through a
separate Go AST provider. It evaluates one unchanged shared policy against two deployment models: an orchestrated
container service and a request-scoped function runtime. Both produce exactly one unresolved error; an unannotated
best-effort timer produces a code fact but no finding.

The foundation test executes both provider processes and asserts that the models load an identical policy and produce
the same stable finding identity:

```sh
go test ./internal/foundation -run TestOneInvariantAcrossTwoDeploymentModels -v
```

## Development

```sh
go test ./...
go vet ./...
```

The fixture scenarios cover durable deferred execution, benign deferred work, replica-local authority, an accepted
architectural choice, and a known analyzer limitation recorded as a false positive. They express semantic inputs and
expected findings rather than carrying forward any source-language scanner implementation.

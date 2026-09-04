# Waldo

Waldo evaluates architectural invariants by combining facts about code with facts about how that code is deployed.

Static analysis can tell you that code schedules deferred work. It cannot, by itself, tell you whether that work
survives the way the process is run. A deployment model can say that processes are restartable and local scheduling
is non-durable. It cannot, by itself, identify which source operation carries a product guarantee. Waldo joins those
facts through an architectural policy and reports the consequence.

## The proof

The repository contains one source-backed invariant:

> Correctness-critical deferred work must not depend on process-local scheduling when its execution authority is
> restartable.

The standalone [example source](examples/durable-deferred-work/app/expiry.go) contains two Go `time.AfterFunc` calls.
A separate [Go AST provider](providers/goast/README.md) reports both as process-local deferred execution, but marks only
the explicitly declared expiry notification as correctness-critical. The best-effort telemetry timer is a code fact,
not a finding.

One unchanged [policy document](policies/durable-deferred-execution.yaml) evaluates that code against two distinct
deployment models:

| Input | Container service | Function runtime |
| --- | --- | --- |
| Code facts | Same provider output | Same provider output |
| Execution model | `orchestrated-container` | `request-scoped-function` |
| Restartable process | `true` | `true` |
| Process-local scheduling durable | `false` | `false` |
| Policy | Same shared file | Same shared file |
| Result | One unresolved error | One unresolved error |

Run the proof from the repository root:

```sh
go run ./cmd/waldo check \
  --root . \
  --config examples/durable-deferred-work/container.waldo.yaml

go run ./cmd/waldo check \
  --root . \
  --config examples/durable-deferred-work/function.waldo.yaml
```

Both commands intentionally exit `1`: each produces the same stable `durable-deferred-execution` finding. The
[foundation test](internal/foundation/foundation_test.go) executes both provider processes and verifies that the two
models load an identical policy and produce an identical finding identity:

```sh
go test ./internal/foundation -run TestOneInvariantAcrossTwoDeploymentModels -v
```

This is the current foundation claim. Waldo is not claiming a broad rule catalog yet.

## How Waldo reaches a finding

Four independent inputs remain visible in the result:

```text
deployment facts + code facts + architectural policy + human disposition -> findings
```

- Deployment facts describe objective properties of deployed units. They contain no source-language semantics.
- Code facts come from replaceable analyzers. They contain no assumptions about the deployment.
- Policies express architectural invariants as joins over both sets of facts.
- Dispositions record a human decision about one stable finding. They do not rewrite facts or weaken a policy.

Waldo core is language- and analyzer-agnostic. OpenGrep, GritQL, compiler-backed analyzers, and small structural
scanners can all emit the same versioned [JSONL provider protocol](docs/provider-protocol.md). The in-repository Go
provider is one proof of that interface; the core binary does not import it.

## Deployment and policy model

The public deployment-model file is `waldo.yaml`. A deployment unit maps source roots to objective facts:

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

## Findings and CI

Severity and disposition are separate:

- Severity: `error | warning | info`
- Disposition: `unresolved | accepted | false-positive`

`waldo check` exits `1` only for unresolved errors. Accepted and false-positive findings remain in the report as
evidence. Configuration, provider, input, and missing fact-source failures exit `2`, so an accidentally unconfigured
CI scan cannot silently pass.

For a deterministic or precomputed scan, bypass configured providers with JSONL facts:

```sh
go run ./cmd/waldo check --root . --config waldo.yaml --facts facts.jsonl --json
```

`waldo compare` separates introduced, resolved, changed, and unchanged findings so a proposed change can be evaluated
without conflating it with existing debt:

```sh
go run ./cmd/waldo compare --base base.report.json --head head.report.json --json
```

## What should come next

The next source-backed proof should test a different architectural dimension: process-local coordination under
multiple instances. It should combine a deployment fact such as `process.replicas > 1` with code facts showing local
state used for locking, deduplication, uniqueness, or another authoritative decision.

A draft `replica-local-authority` policy and semantic fixtures exist, but they are not yet a provider-backed product
claim. General local-state provenance should begin as a warning; narrower evidence of process-local coordination may
justify a separate error-level invariant. The next work is to prove that boundary, not add another timer detector.

## Development

```sh
go test ./...
go vet ./...
```

The fixture scenarios cover durable deferred execution, benign deferred work, replica-local authority, an accepted
architectural choice, and a known analyzer limitation recorded as a false positive. They express semantic inputs and
expected findings rather than carrying forward any source-language scanner implementation.

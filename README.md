# Waldo

Waldo evaluates architectural invariants by combining facts about code with facts about how that code is deployed.

Static analysis can tell you that code schedules deferred work. It cannot, by itself, tell you whether that work
survives the way the process is run. A deployment model can say that processes are restartable and local scheduling
is non-durable. It cannot, by itself, identify which source operation carries a product guarantee. Waldo joins those
facts through an architectural policy and reports the consequence.

## The proofs

The repository contains two source-backed invariants:

> Correctness-critical deferred work must not depend on process-local scheduling when its execution authority is
> restartable.

> Cross-request coordination that requires deployment scope must not depend on process-local authority when multiple
> independently executing instances have instance-scoped memory.

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

The second proof uses a thin [Semgrep provider adapter](providers/semgrep/README.md). A deliberately narrow
provider-side rule recognizes module-local state exposed as a deployment-scoped cross-request predicate and emits an
analyzer-neutral `coordination` fact. The same source produces an unresolved error under the replicated model and no
finding under the single-instance model:

```sh
go run ./cmd/waldo check \
  --root . \
  --config examples/process-local-coordination/replicated.waldo.yaml

go run ./cmd/waldo check \
  --root . \
  --config examples/process-local-coordination/single-instance.waldo.yaml
```

The first command intentionally exits `1`; the second exits `0`. This proof requires Semgrep on `PATH`. The adapter
translates only rules with explicit `metadata.waldo`; ordinary lint and security results are ignored.

These are the current foundation claims. Waldo is not claiming a broad rule catalog.

## How Waldo reaches a finding

Four independent inputs remain visible in the result:

```text
deployment facts + code facts + architectural policy + human disposition -> findings
```

- Deployment facts describe objective properties of deployed units. They contain no source-language semantics.
- Code facts come from replaceable analyzers. They contain no assumptions about the deployment.
- Policies express architectural invariants as joins over both sets of facts.
- Dispositions record a human decision about one stable finding. They do not rewrite facts or weaken a policy.

Waldo core is language- and analyzer-agnostic. OpenGrep, Semgrep, GritQL, compiler-backed analyzers, and small
structural scanners can all emit the same versioned [JSONL provider protocol](docs/provider-protocol.md). The
in-repository Go provider and Semgrep adapter prove that interface; the core binary imports neither.

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

## Policy contribution boundary

Only cross-boundary architectural invariants belong in Waldo core: the conclusion must require both code facts and
deployment facts. A core policy must also be phrased without naming a programming-language API or infrastructure
product. Checks that need no deployment context belong in analyzers; product recommendations belong in integrations.

The full admission test, evidence matrix, and severity criteria are in [CONTRIBUTING.md](CONTRIBUTING.md). The small
current and proposed rule families are tracked in the [policy taxonomy](docs/policy-taxonomy.md).

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

Use real pull requests to select the next invariant. A candidate is eligible only after a concrete source example,
explicit deployment facts, and a conclusion that normal lint cannot reach. Likely areas include narrow local locking,
deduplication, uniqueness, leadership, ephemeral filesystem authority, and volatile buffered delivery. None should be
added merely to fill out the taxonomy.

Stale state reused across an asynchronous yield is not, by itself, a Waldo invariant: it can fail inside one process
with one deployed instance. That belongs in a source analyzer unless a separate architectural conclusion genuinely
requires an explicit deployment fact.

`replica-local-authority` remains a warning. `process-local-coordination` is an error only when the provider explicitly
establishes high confidence and deployment-wide required scope; “local cache under multiple replicas” is not enough.

## Development

```sh
go test ./...
go vet ./...
```

The fixture scenarios cover durable deferred execution, benign deferred work, high-confidence process-local
coordination, low-confidence local state, replica-local authority, an accepted architectural choice, and a known
analyzer limitation recorded as a false positive. They express semantic inputs and expected findings rather than
coupling policy evaluation to a source-language analyzer.

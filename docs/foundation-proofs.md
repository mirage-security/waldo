# Foundation proofs

Waldo currently claims two source-backed invariant families. These examples demonstrate both process boundaries,
the deployment join, policy result, and required counterfactuals. They are design evidence rather than consumer setup.

## Durable deferred execution

The [example source](../examples/durable-deferred-work/app/expiry.go) contains two Go `time.AfterFunc` calls. The
separate [Go AST provider](../providers/goast/README.md) reports both as process-local deferred execution, but marks
only the explicitly declared expiry notification as correctness-critical. The best-effort telemetry timer remains a
code fact without becoming a finding.

The same [policy](../policies/durable-deferred-execution.yaml) evaluates that source against two deployment bindings.
Each binding uses the normalized `facts` adapter to select separate deployment evidence; the code artifact and policy
remain unchanged.

| Input | Container service | Function runtime |
| --- | --- | --- |
| Code facts | Same provider output | Same provider output |
| Execution model | `orchestrated-container` | `request-scoped-function` |
| Restartable process | `true` | `true` |
| Process-local scheduling durable | `false` | `false` |
| Policy | Same shared file | Same shared file |
| Result | One unresolved error | One unresolved error |

Run both models from the repository root:

```sh
go run ./cmd/waldo check \
  --root . \
  --config examples/durable-deferred-work/container.waldo.yaml

go run ./cmd/waldo check \
  --root . \
  --config examples/durable-deferred-work/function.waldo.yaml
```

Both intentionally exit `1` and produce the same stable `service/deployment` finding identity. The executable proof
also verifies that both deployment adapters completed and preserves that claim:

```sh
go test ./internal/foundation -run TestOneInvariantAcrossTwoDeploymentModels -v
```

## Process-local authority

The [process-local authority example](../examples/process-local-authority/) uses three synthetic JavaScript shapes: a
process-local option table, an in-memory source of record, and a replaceable cache backed by durable storage. The
provider reports the same bounded source fact for each because syntax alone cannot establish the role of the state.

The policy joins those facts with a replaceable deployment whose memory is instance-scoped. Developers see one
`process-local-authority` warning and can accept an intentional consequence or mark a durable cache false-positive
without removing the evidence.

```sh
go run ./cmd/waldo check \
  --root . \
  --config examples/process-local-authority/waldo.yaml

go test ./internal/foundation -run TestProcessLocalAuthoritySourceProof -v
```

The foundation test verifies two accepted findings and one false-positive finding. A separate policy matrix removes
the code premise and the deployment premise independently, and confirms that a provider fact establishing
`state.role: cache` produces no finding.

## Process-local coordination

The stricter coordination proof uses the built-in [JavaScript provider](../providers/javascript/README.md), whose current backend is
the separate [Semgrep adapter](../providers/semgrep/README.md). A narrow provider-side rule recognizes module-local
state exposed as a deployment-scoped cross-request predicate and emits one analyzer-neutral `coordination` fact.

```sh
go run ./cmd/waldo check \
  --root . \
  --config examples/process-local-coordination/replicated.waldo.yaml

go run ./cmd/waldo check \
  --root . \
  --config examples/process-local-coordination/single-instance.waldo.yaml
```

The replicated model intentionally exits `1` with `process-local-coordination`. The single-instance model exits `0`
with no finding. Both report that the deployment adapter and the same code provider completed, proving that the zero
result comes from the deployment counterfactual rather than silent analysis.

This proof requires Semgrep on `PATH`. The JavaScript provider owns the backend rule, and the adapter translates only
results carrying explicit `metadata.waldo`; ordinary lint and security results are ignored.

## Interpreting zero findings

A zero result supports only this conclusion:

> No violations were found within the semantics and source coverage this provider establishes.

For each corpus experiment:

1. Run a known positive through the same provider binary, policies, targets, and deployment model.
2. Remove the code premise and confirm the finding disappears.
3. Restore the code premise, remove the deployment premise, and confirm the finding disappears.
4. Inspect deployment-adapter and provider completion and fact counts in both reports.
5. Manually review the relevant code or pull-request diff for assumptions the provider may have missed.

Report schema v5 records deployment-adapter accounting, protocol-v2 provider coverage, optional consumer
recommendations, and the exact code and
deployment attributes that matched each finding. Protocol-v1 providers retain completion and fact counts without
parsed-file coverage.
Protocol v1 does not expose backend-specific discovery or skip telemetry.

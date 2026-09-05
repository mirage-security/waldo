# Waldo

Waldo catches code whose architectural guarantees do not hold in the way that code is deployed.

It combines facts from source analyzers with objective deployment facts. Waldo is not a general-purpose linter: a
core rule belongs here only when its conclusion requires both sides.

```text
deployment facts + code facts + architectural policy + human disposition -> findings
```

## Why Waldo?

Code can be correct in one process and wrong once that process restarts or multiple copies run. A local timer can
disappear before its callback runs; a counter can lose updates across replicas. A source linter sees the APIs but not
the deployment, while an infrastructure check sees the topology but not what the code is trying to guarantee. Waldo
evaluates the mismatch.

The project grew from [an experiment in making infrastructure invisible](https://nickdirienzo.com/an-experiment-in-making-infrastructure-invisible/).
Its name refers to Waldo et al.'s [*A Note on Distributed Computing*](https://waldo.scholars.harvard.edu/publications/note-distributed-computing):
local and distributed systems can share interfaces, but they cannot be assumed to share semantics.

## Quick start

Waldo currently requires Go 1.27.1. JavaScript and TypeScript analysis uses local, token-free Semgrep CE as an internal
backend.

```sh
go install github.com/mirage-security/waldo/cmd/waldo@latest
go install github.com/mirage-security/waldo/cmd/waldo-javascript-provider@latest
```

Install [Semgrep](https://semgrep.dev/docs/getting-started/quickstart) and make sure all three commands are on `PATH`.
Consumers run Waldo, not Semgrep directly.

Add `waldo.yaml` at the repository root:

```yaml
version: 1

deployment:
  units:
    reporting:
      codeRoots:
        - services/reporting/src
      facts:
        process.restartable: true
        process.replicas: 2
        process.instances.concurrent: 2
        memory.scope: instance
        scheduling.processLocal.durable: false
```

Then run:

```sh
waldo check
```

Waldo automatically loads its built-in policies, selects its packaged provider, and derives analysis targets from
`codeRoots`. Consumers do not copy policy files or maintain Semgrep rules for built-in language semantics.

Example output:

```text
Analysis: 1 provider completed; 1 code fact; 1 deployment unit; 4 policies.
  javascript: 1 code fact
WARNING non-durable-deferred-execution This deferred work may never run if the process stops or restarts first. services/reporting/src/jobs.ts:42 [unresolved]
  waldo:v1:...

1 findings: 1 unresolved, 0 accepted, 0 false-positive; 0 failing
```

## Findings and CI

Severity and disposition are independent:

- Severity: `error | warning | info`
- Disposition: `unresolved | accepted | false-positive`

`waldo check` exits `1` only when an unresolved error exists. Warnings remain visible without failing CI. Accepted and
false-positive findings remain in reports as evidence. Configuration, provider, and input failures exit `2`.

Write a machine-readable report with:

```sh
waldo check --json > waldo.report.json
```

Compare a pull request with its base revision using complete reports:

```sh
waldo compare --base base.report.json --head head.report.json
```

Comparison separates introduced, resolved, changed, and unchanged findings. Only newly failing findings fail the
comparison.

Every report records completed providers and normalized fact counts. A zero-finding result is therefore inspectable,
but it is not proof that the codebase has no architectural violations. Calibrate corpus scans with a known positive
and a deployment counterfactual; see [Foundation proofs](docs/foundation-proofs.md).

## How it works

- Deployment units declare source roots and objective properties such as restartability, concurrency, memory scope,
  and durability.
- Replaceable providers translate language and framework behavior into analyzer-neutral code facts.
- Provider-neutral policies join code facts with deployment facts.
- Human dispositions annotate one stable finding without changing facts or policy behavior.

The normal setup is topology-only. Explicit providers and policy files are supported for custom integrations and
focused experiments; they replace the built-in selection rather than extending it.

## Documentation

- [Architecture](docs/architecture.md)
- [Foundation proofs](docs/foundation-proofs.md)
- [Provider protocol](docs/provider-protocol.md)
- [Policy taxonomy](docs/policy-taxonomy.md)
- [Contribution and rule-admission guide](CONTRIBUTING.md)

## Development

```sh
gofmt -w cmd internal protocol providers examples
go test ./...
go vet ./...
go build ./cmd/...
git diff --check
```

## License

Waldo is available under the [MIT License](LICENSE).

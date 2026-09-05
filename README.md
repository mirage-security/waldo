# Waldo

Waldo catches code whose architectural guarantees do not hold when that code is deployed.

```text
deployment facts + code facts + architectural policy + human disposition -> findings
```

A local timer can disappear with its process. A process-local lock cannot coordinate independently running copies.
Source linters see the APIs but not the topology; infrastructure checks see the topology but not what the code is
trying to guarantee. Waldo evaluates the combination.

## Why Waldo?

Code can be correct in one execution model and wrong in another. Waldo's core rule for admitting a policy is therefore
strict: its conclusion must require both a code fact and a deployment fact. Checks that do not need deployment context
belong in ordinary analyzers instead.

The project grew from [an experiment in making infrastructure invisible](https://nickdirienzo.com/an-experiment-in-making-infrastructure-invisible/).
Its name refers to Waldo et al.'s [*A Note on Distributed Computing*](https://waldo.scholars.harvard.edu/publications/note-distributed-computing):
local and distributed systems can share interfaces, but they cannot be assumed to share semantics.

## Quick start

Waldo requires Go 1.27.1. Install the CLI, its JavaScript provider, and the static Terraform deployment adapter:

```sh
go install github.com/mirage-security/waldo/cmd/waldo@latest
go install github.com/mirage-security/waldo/cmd/waldo-javascript-provider@latest
go install github.com/mirage-security/waldo/cmd/waldo-terraform-deployment-adapter@latest
```

JavaScript and TypeScript analysis currently uses local, token-free Semgrep CE as an internal backend. Install
[Semgrep](https://semgrep.dev/docs/getting-started/quickstart) and make sure the commands above are on `PATH`.
Consumers run Waldo, not Semgrep directly.

Place `waldo.yaml` beside the service source:

```yaml
version: 2
service: reporting

artifacts:
  server:
    entrypoint: src/index.ts

deployments:
  production:
    artifact: server
    from:
      adapter: terraform
      source: infra
      resource: module.service
      with:
        varFiles:
          - production.tfvars
```

Then run from the repository root:

```sh
waldo check --config services/reporting/waldo.yaml
```

This file does not repeat Terraform's topology. It binds the `server` artifact to the existing deployment resource:

- `adapter` says how Waldo reads the deployment evidence;
- `source` locates that evidence relative to `waldo.yaml`; and
- `resource` selects the deployable object inside it.

The artifact source defaults to the directory containing `waldo.yaml`. Add `source` to an artifact only when its code
lives in a child directory. Deployment adapters inspect existing files but never execute deployment tools, initialize
providers, read state, contact backends, or access cloud APIs.

Waldo loads its built-in policies and source providers automatically. Explicit policies and providers are advanced
full overrides for focused proofs and custom integrations.

## Findings and CI

Severity and disposition are independent:

- Severity: `error | warning | info`
- Disposition: `unresolved | accepted | false-positive`

`waldo check` exits `1` only for an unresolved error. Warnings remain visible without failing CI. Accepted and
false-positive findings remain in reports as evidence. Configuration, adapter, provider, and input failures exit `2`.

```sh
waldo check --config services/reporting/waldo.yaml --json > waldo.report.json
waldo compare --base base.report.json --head head.report.json
```

Comparison separates introduced, resolved, changed, and unchanged findings. Only newly failing findings fail the
comparison. Reports record completed deployment adapters, completed source providers, and their normalized fact
counts, making an unexpected zero-result scan inspectable.

## Adapters and providers

The same binding contract supports additional static deployment adapters. For example, a Kubernetes adapter can use:

```yaml
from:
  adapter: kubernetes
  source: deploy/rendered/production.yaml
  resource: Deployment/reporting
```

Kubernetes support is not implemented yet; the example shows that adding it does not change the public binding
vocabulary or core policies.

External adapters use the same binding shape:

```yaml
from:
  adapter: ../../tools/waldo-mirage-deployment-adapter
  source: .
  resource: production
```

The external adapter can combine repository-specific conventions and files such as `deploy.json` without teaching
Waldo core about them. The built-in `facts` adapter reads already-normalized deployment evidence and exists as an
explicit escape hatch and executable test fixture—not as the normal consumer format.

Code providers separately translate source syntax, runtime behavior, and framework semantics into code facts. Core
policies know neither deployment products nor programming-language APIs.

## Documentation

- [Architecture](docs/architecture.md)
- [Deployment adapter protocol](docs/deployment-adapter-protocol.md)
- [Foundation proofs](docs/foundation-proofs.md)
- [Code-fact provider protocol](docs/provider-protocol.md)
- [Policy taxonomy](docs/policy-taxonomy.md)
- [Contribution and rule-admission guide](CONTRIBUTING.md)

## Development

```sh
gofmt -w adapters cmd internal protocol providers examples policies
go test ./...
go vet ./...
go build ./cmd/...
git diff --check
```

## License

Waldo is available under the [MIT License](LICENSE).

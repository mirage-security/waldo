# Waldo

Waldo catches code whose architectural guarantees do not hold when that code is deployed.

```text
deployment facts + code facts + architectural policy + human disposition -> findings
```

## Why use Waldo?

A source tool can find a local timer or lock. It usually does not know whether the process can be replaced or how many
copies may run. An infrastructure tool knows how the code is deployed, but not whether the timer schedules required
work or the lock protects shared state. Waldo checks both.

Waldo's scope is narrow. A core policy must require both a code fact and a deployment fact. A check that needs only one
belongs in a source or infrastructure tool.

### Can't I check this another way?

You can build this yourself: run source checks, read the deployment configuration, and join the results. Waldo provides
that workflow, along with stable findings, accepted exceptions, and base/head comparison. It works alongside source
linters, security scanners, and infrastructure checks.

### The name

Waldo is named after Waldo et al.'s
[*A Note on Distributed Computing*](https://waldo.scholars.harvard.edu/publications/note-distributed-computing).
It started with [an experiment in making infrastructure invisible](https://nickdirienzo.com/an-experiment-in-making-infrastructure-invisible/).

## Quick start

Waldo requires Go 1.27.1. Install the CLI, its JavaScript provider, and the deployment adapters you use:

```sh
go install github.com/mirage-security/waldo/cmd/waldo@latest
go install github.com/mirage-security/waldo/cmd/waldo-javascript-provider@latest
go install github.com/mirage-security/waldo/cmd/waldo-terraform-deployment-adapter@latest
go install github.com/mirage-security/waldo/cmd/waldo-kubernetes-deployment-adapter@latest
go install github.com/mirage-security/waldo/cmd/waldo-compose-deployment-adapter@latest
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

[`examples/terraform-ecs-service/`](examples/terraform-ecs-service/) contains an executable version of this pattern
with standalone source, Terraform configuration, and a shared Waldo policy.

[`examples/process-local-authority/`](examples/process-local-authority/) shows how one warning covers local state that
is accepted application behavior, an intentional temporary constraint, or a false-positive because durable storage
remains authoritative.

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
comparison. Reports record completed deployment adapters, source-provider coverage, and normalized fact counts,
making an unexpected zero-result scan inspectable. Partial provider coverage exits `2` unless the research-oriented
`--allow-partial` flag is explicit.

Human and JSON findings include the selected deployment plus the exact code and deployment facts that satisfied the
policy, so the cross-boundary reason for a finding remains inspectable.

## Adapters and providers

Terraform, rendered Kubernetes manifests, and static Docker Compose files are currently supported deployment formats. The executable
[`terraform-ecs-service`](examples/terraform-ecs-service/) example binds an artifact to a raw ECS resource. The
adapter also recognizes the supported Terraform AWS module shapes documented in
[`adapters/terraform/README.md`](adapters/terraform/README.md).

The Kubernetes adapter currently recognizes `apps/v1` Deployments in raw or explicitly rendered YAML:

```yaml
from:
  adapter: kubernetes
  source: deploy/rendered/production.yaml
  resource: Deployment/reporting
  with:
    namespace: production
```

It applies Deployment defaults, including rolling-update overlap, but never renders Helm or Kustomize and never
contacts a cluster. See [`adapters/kubernetes/README.md`](adapters/kubernetes/README.md) for its supported evidence and
limits.

The Compose adapter selects `service/name` from one already-merged YAML file. It reads literal scale settings but does
not invoke Docker Compose, merge overlays, or resolve environment interpolation. See
[`adapters/compose/README.md`](adapters/compose/README.md) for its evidence boundary.

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

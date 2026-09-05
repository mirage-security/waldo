# Deployment-adapter protocol v1

A deployment adapter is an executable that reads existing deployment evidence and emits objective, analyzer-neutral
facts for one selected resource. Adapters never decide whether source code violates a policy.

`waldo.yaml` binds an artifact to a deployment resource:

```yaml
deployments:
  production:
    artifact: server
    from:
      adapter: terraform
      source: infra
      resource: module.service
      with:
        varFiles: [production.tfvars]
```

Names without a path resolve to `waldo-<name>-deployment-adapter` on `PATH`. A relative path resolves from the
directory containing `waldo.yaml`. Waldo writes one JSON request to the adapter's standard input:

```json
{"protocolVersion":1,"root":"/absolute/repository","source":"/absolute/repository/services/reporting/infra","resource":"module.service","options":{"varFiles":["production.tfvars"]}}
```

The adapter writes exactly one JSON object to standard output:

```json
{"facts":{"platform.executionModel":"orchestrated-container","process.restartable":true,"deployment.replicas.concurrent":true,"deployment.replicas.maxConcurrent":2,"memory.scope":"instance","scheduling.processLocal.durable":false}}
```

Standard error is reserved for diagnostics. A non-zero exit, malformed result, null fact map, missing selected
resource, or source outside the analysis root fails the check with exit `2`.

## Static-analysis contract

Adapters inspect artifacts that already exist. They must not silently create those artifacts by running Terraform,
rendering a chart, contacting a control plane, reading live state, or downloading providers. An adapter may expose a
separate opt-in workflow for generated evidence, but `waldo check` must remain deterministic and credential-free.

Unknown is not false. When an adapter cannot establish a property statically, it omits that fact. Policies therefore
cannot match it. Reports record every adapter run and emitted fact count so incomplete coverage is visible.

The Terraform adapter follows checked-in local modules, variable defaults, selected var files, and statically known
locals. It recognizes raw AWS ECS and Lambda resources and the corresponding official modules. It does not attempt to
implement complete Terraform evaluation or infer remote values.

The built-in `facts` adapter reads a small versioned document containing normalized facts. It supports executable
policy matrices and platforms without an adapter:

```yaml
version: 1
resources:
  worker:
    facts:
      process.restartable: true
```

This document is an explicit escape hatch, not a replacement deployment language.

## Trust

An adapter path names executable code. Repositories and CI workflows should execute only adapters they trust, just as
they must trust configured build and test commands. Public-repository privileged workflows should install or pin
trusted adapters outside pull-request-controlled paths.

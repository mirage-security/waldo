# Contributing to Waldo

## The policy admission rule

> A core Waldo policy must express a cross-boundary architectural invariant whose conclusion requires both a code fact
> and a deployment fact.

This is the primary contribution rule. Waldo is not a collection of analyzer rules. OpenGrep, GritQL, compiler-backed
analysis, and language-specific scanners discover code facts; Waldo decides whether those facts have an architectural
consequence in a declared deployment.

A proposed core policy belongs in Waldo only when all of these are true:

1. Removing the code fact makes the conclusion impossible.
2. Removing the deployment fact makes the conclusion impossible.
3. The policy can be phrased without naming a programming-language API or infrastructure product.
4. Providers can normalize concrete syntax into a provider-neutral code fact.
5. Deployment adapters can normalize platform details into objective deployment facts.
6. The finding states an architectural consequence, not a preferred implementation.

If the conclusion still follows without deployment context, it belongs in an analyzer. If it still follows without
code context, it belongs in deployment validation. If it says to use a particular product, that recommendation belongs
outside core policy.

Good invariant:

> Cross-instance exclusion requires authority visible to every participating instance.

Not a core invariant:

> Use Redis instead of a language mutex.

Good invariant:

> Required deferred work must survive the lifetime of its execution instance.

Not a core invariant:

> Do not use a particular timer API.

Not a core invariant:

> Do not reuse state after an asynchronous yield without revalidation.

The source shape alone belongs in an analyzer. A possible core invariant is narrower:

> An atomic read-modify-write operation cannot cross a boundary that admits competing execution against the same
> authority.

That conclusion requires provider facts about the read, dependent write, and suspension; execution and deployment
facts establishing that the suspension admits contenders sharing the authority; and an architectural requirement for
atomicity. A runtime that preserves exclusive ownership through the operation must not produce the finding.

## Policy shape

Every policy should be explainable in this form:

```text
code fact + deployment fact => architectural consequence
```

For example, a future process-local lock policy can be reasoned about as:

```text
process-local exclusion used for deployment-wide coordination
+
multiple independently executing instances with instance-local memory
=
the mechanism cannot provide deployment-wide exclusion
```

An illustrative policy using the current schema would look like this:

```yaml
version: 1

policies:
  - id: process-local-lock
    title: Process-local exclusion cannot coordinate multiple instances
    severity: error
    when:
      deployment:
        process.instances.concurrent:
          greaterThan: 1
        memory.scope: instance
      code:
        kind: exclusion
        attributes:
          exclusion.authority: process-local
          exclusion.requiredScope: deployment
    message: Required deployment-wide exclusion relies on authority visible to only one instance.
```

This example is a design template, not a shipped policy. Concrete APIs belong in providers that emit the neutral
`exclusion` fact.

## Evidence required for a policy

A policy contribution must include an executable evidence matrix:

| Code facts | Deployment facts | Expected result |
| --- | --- | --- |
| Match | Match | Finding with the proposed severity |
| Match | Do not match | No finding |
| Do not match | Match | No finding |

It must also include:

- a semantic explanation of the invariant;
- provider or provider-fixture evidence for the code fact;
- at least one realistic `waldo.yaml` deployment model;
- an explicit severity rationale;
- positive, negative, accepted-disposition, and known-noise scenarios where applicable;
- stable provider fact identities that do not depend on line numbers; and
- a check that the policy and finding message name no language API or infrastructure product.

A second positive deployment model is strongly preferred. It demonstrates that the invariant survives a change in
platform context without modifying or copying the rule.

## Severity

Use `error` only when the joined facts establish a direct contradiction—something close to a theorem. For example, an
instance-local mutex cannot provide exclusion among independently executing instances.

Use `warning` when the joined facts identify architectural risk but intent, provenance, or dataflow is not strong
enough to establish a contradiction. `replica-local-authority` is intentionally warning-level because consulting local
state may be a harmless optimization with a durable fallback.

Use `info` when the result is evidence worth retaining but does not currently require review or block CI.

Human acceptance changes only a finding's disposition. It never changes the source facts, deployment facts, policy,
or severity.

## Fact ownership

- Providers own syntax, runtime semantics, framework behavior, and dataflow. They may recognize specific APIs, but
  should emit normalized facts for policy evaluation.
- Deployment models own objective runtime properties such as restartability, concurrency, memory scope, durability,
  and consistency guarantees.
- Policies own only the cross-boundary invariant.
- Integrations may attach technology-specific remediation, but core finding messages remain product-neutral.

See the [provider protocol](docs/provider-protocol.md), [architecture](docs/architecture.md), and
[policy taxonomy](docs/policy-taxonomy.md) before proposing a new policy.

## Verification

Run the complete suite before submitting a change:

```sh
go test ./...
go vet ./...
go build ./cmd/...
```

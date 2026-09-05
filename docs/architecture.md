# Architecture

Waldo owns orchestration and evaluation, not source-language analysis or deployment-language interpretation.

```text
existing deployment evidence -> deployment adapters -> deployment facts --\
                                                                       policy join -> dispositions -> findings
source code -----------------> code providers -------> code facts ------/
```

## Consumer model

A `waldo.yaml` describes one logical service. Artifacts identify executable source graphs; deployments bind those
artifacts to resources in existing deployment evidence.

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
        varFiles: [production.tfvars]
```

The vocabulary is deliberately small:

- a service is the logical software ownership boundary represented by the model;
- an artifact is something built from one source graph and entrypoint;
- a deployment is one placement or configuration of that artifact; and
- `from` identifies the adapter, source evidence, and selected resource that establish its topology.

The directory containing `waldo.yaml` is an artifact's default source. Multiple artifacts may share a source while
using different entrypoints, and one artifact may be bound to several independently deployed resources.

## Boundaries

- Deployment adapters own Terraform, Kubernetes, Wrangler, platform, and repository-specific deployment semantics.
  They emit objective facts and never evaluate source policy.
- Code providers own source syntax, runtime semantics, framework behavior, package discovery, and dataflow.
- Policies match normalized code facts and deployment facts. Rule IDs and severity remain data.
- Dispositions annotate one stable finding. They do not modify facts, severity, or global behavior.
- Reports retain every finding. CI fails only unresolved errors.

Core launches deployment adapters and code providers through separate versioned process contracts. A custom adapter
may understand repository conventions such as `deploy.json` without introducing that format into Waldo. A custom code
provider may understand house abstractions without making policies application-specific.

Deployment adapters inspect existing artifacts but never create them implicitly. In particular, the Terraform adapter
does not run Terraform, initialize providers, read state, or contact a backend. Unresolved properties remain absent;
policy evaluation never treats an unknown fact as established.

The same rule applies to generated Kubernetes configuration: Waldo reads raw or explicitly rendered manifests but
does not invoke Helm, Kustomize, `kubectl`, or a control plane during `waldo check`.

## Source association

Protocol v1 code providers do not report entrypoint reachability. Waldo invokes built-in providers once per distinct
artifact source and conservatively associates a fact with every deployment whose artifact source contains its path.
The entrypoint records separate executables but does not yet prove that a fact is reachable from only one of several
same-source artifacts.

Scan scope and deployment ownership are separate concerns. Narrowing an artifact source merely to avoid analyzer
output would falsify the source-to-deployment relationship. Precise reachability belongs in a future provider-backed,
analyzer-neutral protocol extension.

## Built-in policy and provider selection

The installed binary embeds Waldo's stable policy catalog and loads it when a model declares no policy override. The
command similarly selects packaged code providers and deduplicates artifact source targets when no providers are
configured.

Explicit policy documents and code providers remain advanced full overrides. Policy paths resolve relative to the
model file. Deployment adapters are selected per deployment binding; names resolve to packaged adapter executables,
while paths allow repository-specific implementations.

Packaged deployment adapters currently normalize selected Terraform resource shapes, rendered Kubernetes
Deployments, and services in an already-merged Docker Compose file. Each adapter owns its format defaults and unknown
states. Core sees only normalized deployment facts and never invokes Terraform, Kubernetes, Helm, or Docker tooling.

## Initial semantic policies

The built-in catalog records four reusable policies across two source-backed invariant families:

1. Correctness-critical deferred work must use durable execution authority when its process can be replaced.
2. Deferred work whose criticality is unknown receives a warning when process-local scheduling can be lost.
3. Application behavior that may depend on state held by one replaceable process deserves warning-level review.
   Providers retain whether that state is known to be authoritative or its role remains unknown.
4. Deployment-scoped coordination cannot use process-local authority when independently executing instances have
   instance-scoped memory. Error severity requires a high-confidence code fact.

These policies do not imply that every timer, cache, or local state value is wrong. Deployment adapters normalize
platform behavior; policies name only the resulting architectural properties.

## Stable identities

Finding IDs are SHA-256 identities over a version tag, policy ID, `service/deployment` identity, provider name, and
provider-owned fact ID. Locations are evidence rather than identity. A provider must keep its fact ID stable across
source movement while changing it when the semantic subject changes.

## Change comparison and auditable zero results

`waldo compare` separates introduced, resolved, changed, and unchanged findings by stable identity. A new unresolved
error fails comparison; an unchanged unresolved error does not.

Report schema v4 records successful deployment-adapter runs, provider coverage, normalized fact counts, deployments,
loaded policies, and the exact code and deployment attributes that satisfied each policy. A failed adapter or
provider prevents report creation and exits `2`. A partial protocol-v2
provider produces an explicitly incomplete report and exits `2` unless `--allow-partial` is set. A successful
zero-fact adapter is visible but does not prove full topology coverage.

Zero-result experiments require a known-positive control through the same adapters, providers, and policies, followed
by counterfactual runs that independently remove the code and deployment premises.

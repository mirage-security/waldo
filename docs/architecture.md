# Architecture

Waldo owns orchestration and evaluation, not source-language analysis.

```text
waldo.yaml deployment units ─┐
provider JSONL code facts ───┼─ generic policy join ─ dispositions ─ findings/report/exit
built-in or explicit policies ───┘
```

## Boundaries

- The deployment model contains objective properties of deployed units and a source description for each unit.
- Providers contain source, language, runtime, framework, and dataflow knowledge. A cheap structural provider is a
  valid choice when a rule does not need heavyweight analysis.
- Policies match typed code facts and deployment facts. Rule IDs and their severities are configuration data.
- Dispositions annotate one stable finding. They do not modify deployment facts, provider facts, severity, or global
  rule behavior.
- Reports retain every finding. CI policy fails only unresolved errors.

The same `source.root` may produce more than one deployment unit. An optional `source.entrypoint`, relative to that
root, records which executable each unit starts. This supports an HTTP service and several workers built from one
project without pretending they share deployment facts.

Protocol v1 providers do not yet report entrypoint reachability. Until a source-backed provider adds that evidence,
Waldo conservatively associates a fact under a shared root with every unit using that root and evaluates a separate
finding for each unit. The entrypoint is therefore part of the public deployment model but not yet a claim that Waldo
can prove a fact is reachable from only one of several same-root executables. Scan scope and deployment ownership are
separate concerns; narrowing `source.root` to avoid analyzer errors would falsify the topology.

The installed binary embeds Waldo's stable policy documents and loads them when a model declares no policy set.
Likewise, the command selects packaged providers and deduplicates source targets from deployment `source.root` values
when a model declares no providers. This makes topology the normal consumer configuration while preserving the
process boundary: the core command launches provider executables through protocol v1 and does not import their source
analyzers.

Explicit policy documents and providers remain advanced full overrides. Policy paths are resolved relative to the
model file, allowing focused proofs and custom integrations to vary independently from the built-in catalog.

## Initial semantic policies

The built-in catalog records four reusable policies across two source-backed invariant families:

1. Correctness-critical deferred work must use durable execution authority when its process is restartable. A timer is
   only one possible analyzer-observed manifestation; the invariant is not tied to a timer API or language.
2. Deferred work whose criticality is not established receives a warning when its process-local scheduling authority
   is non-durable and the execution instance can restart. The warning states the loss mode without claiming the work
   is required.
3. Deployment-scoped cross-request coordination cannot use process-local authority when multiple independently
   executing instances have instance-scoped memory. Error severity requires a high-confidence provider fact.
4. A decision that may treat process-local state as authoritative in a multi-replica deployment deserves warning-level
   review. Providers with stronger provenance or dataflow can produce better facts. More specific locking,
   deduplication, or coordination policies may justifiably be errors.

These policies deliberately do not imply that all deferred work is correctness-critical or all local caches are
architectural failures. The two deferred-execution policies are mutually exclusive for the current providers:
established critical work produces the error, while unknown criticality produces the warning.

The executable foundation proofs use a Go-specific provider and a separate Semgrep adapter. The adapter translates
only analyzer rules with explicit Waldo metadata and leaves parsing, discovery, syntax, and concrete APIs in Semgrep.
The JavaScript provider packages generic language rules behind its provider command and currently delegates to that
adapter internally. Consumers do not configure Semgrep for built-in language facts. Core imports none of these
providers, and another analyzer can replace a backend while emitting the same fact shape.

## Stable identities

Finding IDs are SHA-256 identities over a version tag, policy ID, deployment unit, provider name, and provider-owned
fact ID. Providers must keep a fact ID stable across source movement while changing it when the semantic subject
changes. This supports narrow review decisions without relying on broad symbol allowances.

## Change comparison

`waldo compare` compares base and head reports by stable identity. It separates introduced, resolved, changed, and
unchanged findings, allowing a proposed change to be evaluated independently from existing debt. A new unresolved
error fails comparison; an unchanged unresolved error does not.

## Auditable zero results

Report schema v2 records successful provider runs and their normalized fact counts alongside the number of deployment
units and loaded policies. A provider failure still prevents report creation and exits `2`. Comparison output carries
both reports' accounting, making an unexpected change from “provider emitted facts” to “provider emitted zero facts”
visible even when neither report contains a finding.

This accounting does not prove source coverage. Provider protocol v1 contains semantic facts, not analyzer telemetry
about discovered files, parsed files, or skipped paths. Zero-result experiments therefore require a known-positive
control through the same provider and deployment model, followed by counterfactual runs that independently remove the
matching code and deployment premises. Richer source-coverage telemetry should only extend the public protocol when a
real provider demonstrates a portable contract for it.

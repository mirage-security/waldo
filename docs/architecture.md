# Architecture

Waldo owns orchestration and evaluation, not source-language analysis.

```text
waldo.yaml deployment units ─┐
provider JSONL code facts ───┼─ generic policy join ─ dispositions ─ findings/report/exit
shared policy documents ─────┘
```

## Boundaries

- The deployment model contains objective properties of deployed units and the source roots that compose them.
- Providers contain source, language, runtime, framework, and dataflow knowledge. A cheap structural provider is a
  valid choice when a rule does not need heavyweight analysis.
- Policies match typed code facts and deployment facts. Rule IDs and their severities are configuration data.
- Dispositions annotate one stable finding. They do not modify deployment facts, provider facts, severity, or global
  rule behavior.
- Reports retain every finding. CI policy fails only unresolved errors.

The same source root may belong to more than one deployment unit. Waldo evaluates a separate finding for each unit
because the architectural consequence may differ with deployment facts.

Policy documents can be referenced by multiple `waldo.yaml` files. Paths are resolved relative to the model file,
making reuse explicit and allowing deployment models to vary independently from an invariant.

## Initial semantic policies

The examples record three reusable policies, two of them source-backed invariants:

1. Correctness-critical deferred work must use durable execution authority when its process is restartable. A timer is
   only one possible analyzer-observed manifestation; the invariant is not tied to a timer API or language.
2. Deployment-scoped cross-request coordination cannot use process-local authority when multiple independently
   executing instances have instance-scoped memory. Error severity requires a high-confidence provider fact.
3. A decision that may treat process-local state as authoritative in a multi-replica deployment deserves warning-level
   review. Providers with stronger provenance or dataflow can produce better facts. More specific locking,
   deduplication, or coordination policies may justifiably be errors.

These policies deliberately do not imply that all deferred work is correctness-critical or all local caches are
architectural failures.

The executable foundation proofs use a Go-specific provider and a separate Semgrep adapter. The adapter translates
only analyzer rules with explicit Waldo metadata and leaves parsing, discovery, syntax, and concrete APIs in Semgrep.
Core imports neither provider. Another analyzer can replace either while emitting the same fact shape.

## Stable identities

Finding IDs are SHA-256 identities over a version tag, policy ID, deployment unit, provider name, and provider-owned
fact ID. Providers must keep a fact ID stable across source movement while changing it when the semantic subject
changes. This supports narrow review decisions without relying on broad symbol allowances.

## Change comparison

`waldo compare` compares base and head reports by stable identity. It separates introduced, resolved, changed, and
unchanged findings, allowing a proposed change to be evaluated independently from existing debt. A new unresolved
error fails comparison; an unchanged unresolved error does not.

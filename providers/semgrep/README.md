# Semgrep provider adapter

`waldo-semgrep-provider` runs Semgrep rules and translates only results carrying `metadata.waldo` into protocol v1
code facts. Ordinary lint or security results are ignored. This keeps source syntax and concrete APIs on the provider
side of Waldo's boundary.

```yaml
metadata:
  waldo:
    id: typescript.process-local-state-handoff
    kind: coordination
    symbolMessagePrefix: "waldo-symbol:"
    attributes:
      coordination.authority: process-local
      coordination.confidence: high
      coordination.scope: cross-request
```

Set the Semgrep rule message to the same fixed prefix followed by a metavariable, for example
`message: "waldo-symbol:$STATE"`. The adapter strips the declared prefix from Semgrep's expanded result message. This
works with token-free Semgrep CE, whose CI output may omit the separate `extra.metavars` object. The older
`symbolMetavariable` metadata remains supported when that object is available, but a rule must configure exactly one
symbol source.

The `id`, root-relative source path, and extracted symbol form a stable fact identity; source coordinates are evidence
only. Rules must therefore choose a semantic symbol that is unique per result and stable across line movement.
Duplicate identities fail the provider instead of silently producing unstable findings.

Configure the adapter as an external provider:

```yaml
providers:
  - name: semgrep-typescript
    command:
      - waldo-semgrep-provider
      - --config
      - rules/architecture.yaml
      - --target
      - services/worker
```

Semgrep must be installed separately. Waldo does not embed its parser, source discovery, or rule engine. Add a narrow
provider rule only when its semantic fact is backed by real source evidence; do not use this adapter to turn generic
Semgrep findings into core policies.

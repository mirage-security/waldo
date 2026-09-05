# Code-fact provider protocol v1

A provider is an executable speaking protocol v1. Waldo automatically launches its packaged providers for the normal
topology-only configuration. Advanced consumers can replace that selection with explicit argument vectors in
`waldo.yaml`:

```yaml
providers:
  - name: opengrep
    command: [waldo-opengrep-provider, --rules, rules]
```

Waldo starts the command with the analyzed root as its working directory. It writes one request object followed by a
newline to standard input:

```json
{"protocolVersion":1,"root":"/absolute/source/root"}
```

The provider writes one JSON object per line to standard output. Standard error is reserved for diagnostics. A
non-zero exit or malformed fact fails the scan.

```json
{"id":"deferred:checkout-expiry","kind":"deferred-execution","source":{"path":"src/expiry.go","line":41,"column":3},"symbol":"expireCheckout","attributes":{"correctness.critical":true,"execution.authority":"process-local","execution.mechanism":"process-local-timer"}}
```

Required fields are:

- `id`: stable semantic identity unique within the provider. It must not include a line number.
- `kind`: provider-neutral fact kind consumed by policy.
- `source.path`: absolute path under the requested root or a root-relative path.

`source.line`, `source.column`, `symbol`, and `attributes` are optional evidence. Provider names are supplied by the
Waldo configuration and override any `provider` value emitted by the process.

Go providers may import `github.com/mirage-security/waldo/protocol` for the versioned request and fact types. Providers
in every other language use the same JSON contract; the Go package is a convenience, not a required SDK.

Provider selection is policy-specific. A structural matcher can emit `deferred-execution` facts; richer provenance or
dataflow may warrant OpenGrep, Semgrep, or a language-specific analyzer. The transport contract does not privilege one
engine. The in-repository [Semgrep adapter](../providers/semgrep/README.md) demonstrates how analyzer-specific results
become normalized facts without teaching Waldo core about source syntax.

Protocol v1 deliberately has no analyzer-summary record. Core records whether each provider process completed and how
many normalized facts it emitted, but it cannot infer how many source files a provider discovered, parsed, or skipped.
Consumers must calibrate a zero-fact run with a known positive. A future protocol revision may add coverage telemetry
only after multiple providers can express it without leaking backend-specific output into core.

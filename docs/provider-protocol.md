# Code-fact provider protocol v1

A provider is an executable configured as an argument vector in `waldo.yaml`:

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
dataflow may warrant OpenGrep or a language-specific analyzer. The transport contract does not privilege one engine.

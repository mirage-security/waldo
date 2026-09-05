# Code-fact provider protocols

A provider is an executable speaking protocol v1 or v2. Waldo automatically launches its packaged providers for the
normal topology-only configuration. Advanced consumers can replace that selection with explicit argument vectors in
`waldo.yaml`. Explicit providers default to v1 for compatibility; set `protocolVersion: 2` when the provider emits
coverage:

```yaml
providers:
  - name: opengrep
    protocolVersion: 2
    command: [waldo-opengrep-provider, --rules, rules]
```

Waldo starts the command with the analyzed root as its working directory. It writes one request object followed by a
newline to standard input:

```json
{"protocolVersion":1,"root":"/absolute/source/root"}
```

## Protocol v1

A v1 provider writes one code-fact JSON object per line to standard output. Standard error is reserved for diagnostics.
A non-zero exit or malformed fact fails the scan.

```json
{"id":"deferred:checkout-expiry","kind":"deferred-execution","source":{"path":"src/expiry.go","line":41,"column":3},"symbol":"expireCheckout","attributes":{"correctness.critical":true,"execution.authority":"process-local","execution.mechanism":"process-local-timer"}}
```

Required fields are:

- `id`: stable semantic identity unique within the provider. It must not include a line number.
- `kind`: provider-neutral fact kind consumed by policy.
- `source.path`: absolute path under the requested root or a root-relative path.

`source.line`, `source.column`, `symbol`, and `attributes` are optional evidence. Provider names are supplied by the
Waldo configuration and override any `provider` value emitted by the process.

Protocol v1 has no analyzer-summary record. Core records whether each provider process completed and how many
normalized facts it emitted, but it cannot infer how many source files a provider discovered, parsed, or skipped.
Consumers must calibrate a zero-fact v1 run with a known positive.

## Protocol v2

Protocol v2 changes the request's `protocolVersion` to `2` and wraps facts in typed records:

```json
{"type":"fact","fact":{"id":"deferred:checkout-expiry","kind":"deferred-execution","source":{"path":"src/expiry.ts","line":41},"symbol":"expireCheckout","attributes":{"execution.authority":"process-local"}}}
```

The final record is one required provider summary:

```json
{"type":"summary","summary":{"coverage":"partial","filesAttempted":2154,"filesNotFullyAnalyzed":88}}
```

`coverage` is `complete` or `partial`. File counts are optional because not every analyzer uses files as its
coverage unit. A summary must not claim complete coverage with a non-zero `filesNotFullyAnalyzed` count.

Waldo retains facts from a partial provider and writes their findings and coverage into the report, but `waldo check`
exits `2` by default. `--allow-partial` is an explicit research workflow that permits the usual finding-based exit
status while keeping partial coverage visible in human and JSON reports.

Go providers may import `github.com/mirage-security/waldo/protocol` for the versioned request, record, summary, and
fact types. Providers in every other language use the same JSON contract; the Go package is a convenience, not a
required SDK.

Provider selection is policy-specific. A structural matcher can emit `deferred-execution` facts; richer provenance or
dataflow may warrant OpenGrep, Semgrep, or a language-specific analyzer. The transport contract does not privilege one
engine. The in-repository [Semgrep adapter](../providers/semgrep/README.md) demonstrates how analyzer-specific results
become normalized facts without teaching Waldo core about source syntax.

Neither protocol carries artifact entrypoints or provider-produced reachability. Waldo invokes built-in
providers once per distinct artifact source and conservatively joins a fact to every deployment whose artifact source
contains the fact's path. An artifact entrypoint records separate executables, but distinguishing which same-source
executable can reach a fact requires a future provider-backed protocol extension. That extension must carry an
analyzer-neutral reachability relation; it must not teach core how a particular language resolves imports.

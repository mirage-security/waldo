# Go AST provider

This provider is a small executable proof of Waldo's analyzer boundary. It uses `go list` for package discovery and
Go's standard AST parser to emit `deferred-execution` facts for `time.AfterFunc` calls. Waldo core imports neither
this package nor Go AST APIs.

The provider emits `execution.authority: process-local` for `time.AfterFunc`; the deployment model, not the provider,
declares whether that authority is durable after deployment. The provider sets `correctness.critical: true` only when
the enclosing function has the exact doc-comment directive:

```go
// waldo:correctness-critical-deferred-work
func ScheduleExpiry(...) {
    time.AfterFunc(...)
}
```

That explicit classification prevents the provider from declaring every timer correctness-critical. Unannotated
timers are still emitted as facts with `correctness.critical: false`, so policy—not the analyzer—decides whether they
produce findings.

This is intentionally a bounded provider, not a general Go dataflow engine. It resolves ordinary named imports of
`time`, analyzes files selected by `go list`, and gives multiple calls within one function an occurrence suffix. More
advanced provenance belongs in a stronger provider behind the same protocol.

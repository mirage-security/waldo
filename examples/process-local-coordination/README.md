# Process-local coordination proof

This example isolates the source shape that motivated Waldo's second invariant. One request writes module-local state
and another request consults it as a predicate. The built-in JavaScript provider normalizes that syntax to a
high-confidence `coordination` fact; its Semgrep backend remains an implementation detail and makes no deployment
judgment.

The same artifact and policy are evaluated against two deployment bindings through the `facts` adapter:

- `replicated.waldo.yaml` declares three concurrently executing instances with instance-scoped memory and produces an
  unresolved error;
- `single-instance.waldo.yaml` declares one instance and produces no finding.

Semgrep must be installed and available on `PATH`:

```sh
go run ./cmd/waldo check --root . --config examples/process-local-coordination/replicated.waldo.yaml
go run ./cmd/waldo check --root . --config examples/process-local-coordination/single-instance.waldo.yaml
```

The first command intentionally exits `1`; the second exits `0`. The syntax rule is deliberately narrow. A module
cache with a durable fallback and a function-local collection are negative fixtures in `providers/semgrep/testdata`.

# Durable deferred-work foundation proof

This example reproduces the durable deferred-execution invariant from standalone Go source. The Go AST provider emits
two facts from [`app/expiry.go`](app/expiry.go): one explicitly correctness-critical timer and one best-effort timer.
Only the critical timer matches the shared policy.

Run the container deployment model from the repository root:

```sh
go run ./cmd/waldo check \
  --root . \
  --config examples/durable-deferred-work/container.waldo.yaml
```

Run the same code and unchanged policy against the function-runtime model:

```sh
go run ./cmd/waldo check \
  --root . \
  --config examples/durable-deferred-work/function.waldo.yaml
```

Both commands report exactly one unresolved `durable-deferred-execution` error and therefore exit `1`. Each model
binds the same artifact and deployment identity to different evidence through the `facts` adapter. The evidence
differs in platform execution model and scale but normalizes the properties relevant to the rule: the process is
restartable and process-local scheduling is non-durable. Both files reference the same policy document
at [`../../policies/durable-deferred-execution.yaml`](../../policies/durable-deferred-execution.yaml); the rule is not
copied or specialized per platform. The `facts` adapter is used here as an executable proof fixture; normal consumers
bind artifacts to their existing Terraform, Kubernetes, or other deployment definitions.

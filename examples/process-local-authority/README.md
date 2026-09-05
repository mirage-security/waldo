# Process-local authority proof

This example contains three deliberately synthetic source shapes:

- process-local runtime options;
- an in-memory draft store; and
- a replaceable catalog cache whose durable source is outside the process.

The JavaScript provider can establish that all three use module-local mutable state across calls, but it cannot prove
their architectural intent. It therefore emits `state.role: unknown` facts. The `process-local-authority` policy joins
those facts with a replaceable deployment whose memory is instance-scoped and reports one warning name for all three.

The model accepts the runtime-option and draft-store consequences. It marks the catalog cache false-positive because
losing it does not lose authoritative state. All three findings and their reasons remain in the report.

Run the example from the repository root:

```sh
go run ./cmd/waldo check --root . --config examples/process-local-authority/waldo.yaml
```

Warnings do not fail the command. `internal/foundation/TestProcessLocalAuthoritySourceProof` verifies that all three
dispositions remain visible.

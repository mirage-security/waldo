# Terraform-backed ECS example

This example binds standalone application source to an existing Terraform resource. The Terraform deployment adapter
reads `infra/main.tf` and `infra/production.tfvars` statically; it does not execute Terraform or contact AWS.

The Go provider observes correctness-critical deferred work using process-local scheduling. The Terraform adapter
observes a restartable ECS deployment whose local scheduler is non-durable. The shared
`durable-deferred-execution` policy joins those facts and reports one unresolved error.

From the repository root, install the adapter and run the example:

```sh
go install ./cmd/waldo-terraform-deployment-adapter
go run ./cmd/waldo check \
  --root . \
  --config examples/terraform-ecs-service/waldo.yaml
```

The check exits `1` because the example intentionally violates the invariant. Its analysis accounting reports one
completed Terraform deployment adapter, one completed Go provider, and one finding.

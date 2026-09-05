# Terraform deployment adapter

This adapter reads Terraform configuration statically. It does not execute Terraform, initialize providers, read
state, contact a backend, or access cloud APIs.

```yaml
from:
  adapter: terraform
  source: infra
  resource: module.service
  with:
    varFiles:
      - production.tfvars
```

The adapter follows checked-in local modules and statically known variables and locals. It currently recognizes raw
AWS ECS services, AWS Lambda functions, and the corresponding official Terraform AWS modules. Unsupported resources
and unresolved expressions produce no guessed facts. A successful zero-fact result remains visible in Waldo's
analysis accounting.

For ECS replica services, the adapter includes rolling-deployment overlap. When `deployment_maximum_percent` is
omitted, it uses ECS's documented [200% default](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DeploymentConfiguration.html).

Install the adapter beside `waldo`:

```sh
go install github.com/mirage-security/waldo/cmd/waldo-terraform-deployment-adapter@latest
```

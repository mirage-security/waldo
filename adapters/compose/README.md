# Docker Compose deployment adapter

The Compose adapter reads one existing Compose YAML file and selects a service by `service/name`. It emits facts about
the service's container process, instance-local memory, process-local scheduling durability, and statically known
replica count.

Use it from `waldo.yaml`:

```yaml
deployments:
  self-hosted:
    artifact: server
    from:
      adapter: compose
      source: compose.yaml
      resource: service/server
```

The adapter uses `scale` or `deploy.replicas` when either is a literal non-negative integer. When both are present,
they must agree. An omitted scale means one container. Interpolated or otherwise unknown values leave the concurrency
facts absent rather than turning unknown into false. Modes other than `replicated` likewise leave concurrency
unknown, and a `container_name` combined with more than one replica is rejected as an invalid Compose model.

`process.restartable: true` describes the service container's replaceability, not its automatic restart policy. A
Compose deployment can recreate a service container even when the optional `restart` field is absent or set to
`"no"`.

The adapter intentionally reads one already-merged YAML document. It does not invoke `docker compose`, merge multiple
`-f` files, resolve profiles or environment interpolation, inspect a Docker engine, or deploy anything. If production
uses overlays or generated configuration, render the exact effective model explicitly before running Waldo and place
that evidence under the analysis root.

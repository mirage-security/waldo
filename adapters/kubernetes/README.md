# Kubernetes deployment adapter

The Kubernetes adapter reads checked-in or explicitly rendered YAML and selects one object by `Kind/name`. Its first
supported shape is an `apps/v1` `Deployment`.

For a Deployment, the adapter emits the common orchestrated-container facts and applies Kubernetes API defaults that
affect process concurrency:

- omitted `spec.replicas` means one replica;
- omitted `spec.strategy.type` means `RollingUpdate`;
- rolling updates can overlap through terminating pods even when `maxSurge` is zero; and
- `Recreate` does not add rollout overlap.

That means even a one-replica rolling Deployment emits `deployment.replicas.concurrent: true`. Kubernetes can leave
terminating pods alive while replacements start, including when `maxSurge` is zero, so the adapter deliberately omits
`deployment.replicas.maxConcurrent` for rolling updates rather than claiming a false ceiling. `Recreate` deployments
emit the configured replica count as their maximum.

Use it from `waldo.yaml`:

```yaml
deployments:
  production:
    artifact: server
    from:
      adapter: kubernetes
      source: deploy/rendered/production.yaml
      resource: Deployment/reporting
      with:
        namespace: production
```

`source` may be one YAML file or a directory, which is searched recursively for `.yaml` and `.yml` files.
Multi-document files and `List` objects are supported. `with.namespace` is optional, but required when the same
`Kind/name` occurs in multiple namespaces.

The adapter does not render Helm or Kustomize, run `kubectl`, query a cluster, or apply manifests. Rendered output must
already exist under the analysis root. Unsupported objects are selected successfully but emit zero facts so that the
coverage gap remains visible in Waldo's adapter accounting.

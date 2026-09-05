# Open-source field study

This document records an exploratory field study of Waldo against code that was not written for Waldo. It is a study
log, not a benchmark or a product claim.

## Research question

Does joining source facts with real deployment evidence produce a finding that a maintainer considers useful and that
would not follow from either side alone?

The study has three independent gates:

1. **Source feasibility:** the packaged provider completes without silently skipping unsupported source.
2. **Deployment feasibility:** an adapter derives facts from checked-in deployment evidence without a researcher
   restating the topology in `waldo.yaml`.
3. **Finding utility:** a maintainer can classify a joined finding as actionable, accepted risk, or false positive.

A repository that fails an earlier gate is not counted as a clean scan or as evidence that its code has no findings.

## Method

- Freeze every repository at a commit before inspecting results.
- Use Waldo's existing JavaScript provider and built-in policies without adding repository-specific source rules.
- Use the Semgrep Python distribution `1.173.0`, the version pinned by Waldo CI at the time of the study. Its
  bundled scan output identifies the engine as `1.104.0`.
- Preserve provider parse failures. Do not convert a partial scan into a zero-fact result.
- Treat existing Terraform, Docker Compose, Kubernetes, and platform configuration as deployment evidence. Do not infer
  production topology from a project's popularity or framework.
- Keep researcher-authored normalized facts separate from adapter-backed results. The `facts` adapter can support a
  policy experiment, but it does not validate normal setup or deployment interpretation.
- Review every candidate in context before showing it to a maintainer. A syntactic match is not a useful finding.

The initial corpus is intentionally TypeScript-heavy because JavaScript/TypeScript is Waldo's only general built-in
source provider. Adding unsupported languages merely to make the sample look diverse would manufacture misleading
zeroes. Within that constraint, the corpus is stratified across serverless applications, long-lived services, and
worker-oriented systems.

## Round 0: feasibility screen

Date: 2026-09-05

| Repository | Commit | Stratum and checked-in deployment evidence | Source scope | Files attempted | Provider outcome | Adapter at screen |
| --- | --- | --- | --- | ---: | --- | --- |
| [Cal.com](https://github.com/calcom/cal.diy) | `abffde336e74` | Next.js; self-host Compose explicitly recommended only for personal, non-production use | `apps/web`, `packages` | 3,972 | Failed: 82 partially parsed files | Unsupported |
| [Dub](https://github.com/dubinc/dub) | `4b40951c3bbe` | Next.js; Vercel configuration and self-hosting instructions; no Compose | `apps/web`, `packages` | 4,180 | Failed: 39 partially parsed files | Unsupported |
| [Outline](https://github.com/outline/outline) | `4533038cb4a1` | Long-lived Node processes; root Compose starts development Redis and Postgres only, not Outline | `server` | 901 | Failed: 6 partially parsed files | Unsupported |
| [Twenty](https://github.com/twentyhq/twenty) | `2a69d83fb1dc` | Separate server and worker; official production/self-host Compose | `packages/twenty-server` | 7,199 | Failed: 159 partially parsed files | Unsupported |
| [n8n](https://github.com/n8n-io/n8n) | `33eb5c196e0c` | Checked-in self-host starter Compose; production-grade and queue-mode setup is documented separately | `packages/cli/src`, `packages/core/src`, `packages/workflow/src` | 2,154 | Failed: 88 partially parsed files; 3 candidate facts underneath | Unsupported |
| [Trigger.dev](https://github.com/triggerdotdev/trigger.dev) | `0a23814a0896` | Separate webapp and workers; self-host Compose plus a production-oriented Kubernetes chart | `apps/webapp`, `apps/supervisor`, `packages` | 2,450 | Failed: 117 partially parsed files | Unsupported |

The requested `calcom/cal.com` repository resolved to `calcom/cal.diy` when cloned. "Files attempted" is Semgrep's
reported target count after the provider's normal test/spec exclusions, not the repository's total file count.

Five repositories contain at least one Compose file, but file presence is a poor adapter-priority metric. Twenty and
Trigger.dev provide the strongest deployment-oriented evidence. Cal.diy and n8n provide runnable self-host stacks
with explicit production caveats. Outline's root Compose file is development support infrastructure and does not
deploy the analyzed application. Dub has no Compose file. A Compose adapter would therefore have four plausible
artifact bindings in this corpus, only two of which are strong production-oriented study subjects.

### Initial result

Normal Waldo execution completed on **0 of 6 repositories**.

Two independent product constraints caused that result:

- The JavaScript provider rejects the whole provider run when Semgrep reports any partial parse error. This is safe
  failure behavior, but modern TypeScript made every corpus member fail.
- At the start of the screen, Waldo's packaged deployment adapter understood Terraform-backed ECS and Lambda shapes.
  The usable deployment evidence in this corpus is Docker Compose, Vercel, or Kubernetes; one additional Compose file
  is local development infrastructure rather than evidence for the analyzed artifact.

This round therefore provides no precision or recall estimate. It does establish that a likely open-source evaluator
cannot reach a finding through the current default path, even when the project has source and deployment documentation.

## Kubernetes adapter prototype

The first adapter follow-up used Trigger.dev's
[official Helm chart](https://github.com/triggerdotdev/trigger.dev/tree/0a23814a0896/hosting/k8s/helm) at the same frozen
commit. The chart was explicitly rendered with its `values-production-example.yaml` file and Helm
`--namespace trigger`. Rendering remained a separate research step; the prototype adapter only read the resulting
YAML directory.

The adapter selected both `apps/v1` Deployments by `Kind/name`:

| Resource | Desired replicas | Rollout evidence | Emitted topology |
| --- | ---: | --- | --- |
| `Deployment/field-study-trigger-webapp` | 1 | Strategy omitted, so `RollingUpdate` applies | Concurrent replicas; numeric maximum unknown |
| `Deployment/field-study-trigger-supervisor` | 1 | `RollingUpdate` with `maxSurge: 1` | Concurrent replicas; numeric maximum unknown |

Both also emitted `orchestrated-container`, restartable process, instance-scoped memory, and non-durable
process-local scheduling facts. This is enough deployment evidence for the existing durability and process-local
coordination policy families without adding Kubernetes names to those policies.

The numeric maximum is intentionally absent. Kubernetes
[does not count terminating pods](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#updating-a-deployment)
against the normal `replicas + maxSurge` calculation, so a terminating process may remain alive while its replacement
starts. The manifest proves that process overlap is possible but does not prove a hard upper bound.

This prototype removes the deployment-feasibility blocker for one production-oriented corpus member. At this stage,
the JavaScript provider still failed on partial parse errors, so no joined finding was claimed.

## Provider-coverage prototype

Provider protocol v2 adds a required final coverage record. The JavaScript provider now retains facts from files that
Semgrep analyzed while reporting warning-level parse gaps by file. Other analyzer errors remain fatal. Waldo writes
the partial report but exits `2` unless the research workflow explicitly sets `--allow-partial`.

Rerunning the frozen corpus produced:

| Repository | Coverage | Files attempted | Files not fully analyzed | Facts retained |
| --- | --- | ---: | ---: | ---: |
| Cal.com | Partial | 3,972 | 82 | 0 |
| Dub | Partial | 4,180 | 39 | 0 |
| Outline | Partial | 901 | 6 | 0 |
| Twenty | Partial | 7,199 | 159 | 0 |
| n8n | Partial | 2,154 | 88 | 3 |
| Trigger.dev | Partial | 2,450 | 117 | 0 |

The n8n candidates are now ordinary provider output rather than a diagnostic bypass. Trigger.dev's zero remains
explicitly partial and therefore is not evidence that the repository has no matching source facts.

This resolves the source-process completion failure without erasing coverage uncertainty. It left n8n as the shortest
path to an external join because the repository has both retained source candidates and a checked-in starter Compose
service.

## Docker Compose adapter prototype

The second adapter follow-up selected `service/n8n` from n8n's checked-in
[`docker/get-n8n-compose.yml`](https://github.com/n8n-io/n8n/blob/33eb5c196e0ce3a2c71525929a4ef861cb94b168/docker/get-n8n-compose.yml).
The adapter read that file directly and did not invoke Docker Compose or fill in environment values.

The service has no explicit scale, so the static Compose model establishes one current container. The adapter emitted:

- `platform.executionModel: composed-container`;
- `process.restartable: true`;
- `memory.scope: instance`;
- `scheduling.processLocal.durable: false`;
- `deployment.replicas.concurrent: false`; and
- `deployment.replicas.maxConcurrent: 1`.

Restartability here means the service container can be recreated by a deployment, independently of Compose's optional
automatic `restart` policy. The adapter treats interpolated scale as unknown, requires `scale` and `deploy.replicas`
to agree when both are present, and expects one already-merged YAML file rather than silently applying overlays.

An end-to-end `waldo check --allow-partial` then joined the 3 retained provider facts with these 6 adapter facts and
the 4 built-in policies. It produced 3 unresolved `non-durable-deferred-execution` warnings, one for each candidate
below. The ordinary report records partial provider coverage: 2,154 files attempted and 88 not fully analyzed.

This is the first successful external join in the study, but it is deliberately not a production claim. n8n presents
this file as a self-host starter, while its production-grade and queue-mode guidance introduces other topology. The
result proves the integration path and supplies a concrete review candidate; it does not prove that every n8n
deployment has one replica or that the warning is actionable.

## Candidate review: n8n

The protocol-v2 JavaScript provider retained three `deferred-execution` facts alongside its explicit partial-coverage
summary. Each became a warning when joined to the starter Compose service. The provider does not infer whether the
callback is correctness-critical, so maintainer context remains part of classifying each warning.

| Candidate | Preliminary classification | Reason |
| --- | --- | --- |
| [`forceShutdownTimer`](https://github.com/n8n-io/n8n/blob/33eb5c196e0ce3a2c71525929a4ef861cb94b168/packages/cli/src/commands/base-command.ts#L521-L543) | Likely noise | The callback is a fallback whose only purpose is to terminate a still-running process during shutdown. Losing it because the process has already stopped is expected. |
| [`ExternalSecretsRetryManager`](https://github.com/n8n-io/n8n/blob/33eb5c196e0ce3a2c71525929a4ef861cb94b168/packages/cli/src/modules/external-secrets.ee/retry-manager.service.ts#L63-L90) | Maintainer review | A failed provider connection is retried only by a process-local timer. Startup appears to initialize providers again, so a restart may safely reconstruct the attempt, but that requires owner confirmation. |
| [test-webhook timeout](https://github.com/n8n-io/n8n/blob/33eb5c196e0ce3a2c71525929a4ef861cb94b168/packages/cli/src/webhooks/test-webhooks.ts#L452-L470) | Confirmed architectural risk | The timer is the only automatic path that turns the cached registration back into the third-party deletion call. Process loss discards the timer; cache expiry only removes the information needed to perform that deletion. |

The last candidate is the best current test of Waldo's thesis. Source analysis can observe the timer but cannot know
that the service runs in a replaceable container. Deployment analysis can observe replacement but cannot know that an
in-process callback tears down an externally registered webhook. The concern requires both.

### Test-webhook trace

Inspection of the frozen source establishes the consequence without relying on naming alone:

1. `needsWebhook` creates a two-minute process-local timer whose callback calls `cancelWebhook`.
2. Before invoking the node's create hook, n8n writes the full test-webhook registration to `CacheService`; it writes
   the registration again after the third-party hook succeeds.
3. The starter Compose service does not configure queue mode or Redis. `CacheService` therefore selects its in-memory
   backend, so both the timer and registration disappear with the process.
4. `cancelWebhook` requires that cached registration to reconstruct the workflow. It calls `deactivateWebhooks`,
   which dispatches the node's `delete` hook and only then deregisters the cached entry.
5. `TestWebhooks` has no startup reconciliation or shutdown handler. Graceful replacement does not close these
   registrations either.
6. This deletion is not merely local bookkeeping. The
   [Telegram trigger](https://github.com/n8n-io/n8n/blob/33eb5c196e0ce3a2c71525929a4ef861cb94b168/packages/nodes-base/nodes/Telegram/TelegramTrigger.node.ts#L234-L292)
   calls Telegram's `setWebhook` and `deleteWebhook` APIs, while the
   [Facebook Lead Ads trigger](https://github.com/n8n-io/n8n/blob/33eb5c196e0ce3a2c71525929a4ef861cb94b168/packages/nodes-base/nodes/FacebookLeadAds/FacebookLeadAdsTrigger.node.ts#L154-L209)
   explicitly creates and deletes an app webhook subscription. Neither implementation establishes self-expiration.

The multi-main TTL is not a durable cleanup executor. It expires the shared cache key after the creator crashes, but
key expiry cannot invoke the third-party `delete` hook. It actually removes the information a later reconciler would
need. Thus a process replacement during the test window can leave a persistent remote subscription targeting a test
route that no longer has a registration. A later test or workflow activation may overwrite it, but the source does
not establish any bounded recovery.

This is sufficient to retain the Waldo finding as a real architectural risk. The built-in JavaScript provider still
correctly reports `correctness.criticality: unknown`: its generic timer match cannot prove which node type will be used
at runtime, and some node deletion hooks are no-ops. Promoting every such timer to an error would overstate the
provider evidence. The field-study trace validates the warning and its prioritization, not a new core policy.

The study also exposed a presentation problem: a human-readable warning showed its source location but not the exact
cross-boundary premises. Waldo now records `matchedCodeFacts` alongside `matchedDeploymentFacts` and prints the
selected deployment and both evidence sets. The policy and stable finding identity remain unchanged.

## What this says today

- Waldo now completes its source-provider, deployment-adapter, and policy-join path on one external repository when
  partial coverage is explicitly allowed.
- The result is evidence of technical feasibility, not yet evidence of maintainer utility or a precision estimate.
- The cross-boundary question surfaced a non-obvious, review-worthy n8n case that requires both a replaceable process
  and an in-process cleanup callback to explain.
- The deployment evidence is a self-host starter, so its production caveats remain part of every finding review.
- Adding more invariant families before resolving those gates would make the study harder to interpret.

## Next rounds

### Round 1: upstream feedback

The source trace is sufficient for Waldo's technical validation; maintainer feedback is no longer a gate. If the
candidate is shared upstream, ask:

1. Can a single-main production process be replaced while a test webhook is registered?
2. Is there an operational cleanup path outside this source tree that independently removes the registration?
3. Would startup reconciliation, shutdown cleanup, or a durable cleanup job best fit n8n's intended behavior?
4. Would the joined explanation have shortened the investigation compared with seeing the timer alone?

Record any answer as additional product feedback. Do not tune the generic provider around n8n-specific names or APIs.

### Round 2: startup codebase

For the planned private startup trial, capture:

- source revision and the exact artifact source/entrypoint;
- the deployment evidence revision and selected resource;
- setup time until the first complete report;
- provider and adapter completion accounting;
- every finding and its owner-supplied disposition;
- whether the owner learned something they did not already know; and
- whether they changed code, deployment, documentation, or nothing.

The strongest initial success criterion is one complete end-to-end run that produces a maintainer-confirmed,
cross-boundary finding with tolerable setup cost. One friendly trial can validate the workflow and reveal failure
modes; it cannot establish a general precision rate.

### Round 3: broader corpus

Repeat the frozen corpus as provider syntax coverage changes. Add other languages only when a general provider can
analyze ordinary external code without requiring Waldo-specific annotations. Add Docker Compose or platform adapters
based on observed deployment evidence; do not encode those semantics into core or `waldo.yaml`.

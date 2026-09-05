# Policy taxonomy

Waldo keeps its core taxonomy deliberately small. Categories describe architectural dimensions, not analyzer engines,
programming languages, or infrastructure products.

## Durability

| Policy | Status | Intended claim |
| --- | --- | --- |
| `durable-deferred-execution` | Source-backed | Required deferred work must survive the lifetime of a restartable execution instance. |
| `non-durable-deferred-execution` | Source-backed warning | Deferred work of unknown criticality may be lost when it relies on process-local scheduling in a restartable instance. |
| `durable-buffered-delivery` | Future | A successful write to a local buffer cannot establish durable delivery when that buffer is lost with the instance. |
| `ephemeral-filesystem-authority` | Future | Instance-ephemeral files cannot serve as durable authoritative state. |

The two source-backed deferred-execution policies express different evidence strengths for the same durability
invariant. Proven correctness-critical work is an error; work whose criticality remains unknown is a warning. They do
not turn every timer into an error. New durability work should broaden provider coverage for the invariant before
adding API-shaped variants of the same rule.

## Coordination

| Policy | Status | Intended claim |
| --- | --- | --- |
| `replica-local-authority` | Draft warning | A correctness path consults instance-local mutable state while multiple instances execute. |
| `process-local-coordination` | Source-backed error | High-confidence process-local authority cannot provide required deployment-wide coordination among concurrent instances. |
| `process-local-lock` | Future specialization | Instance-local exclusion cannot provide fleet-wide exclusion. |
| `process-local-deduplication` | Future error | Instance-local state cannot provide deployment-wide deduplication. |
| `process-local-uniqueness` | Future error | Instance-local state cannot enforce uniqueness across independently executing instances. |
| `process-local-leadership` | Future error | Instance-local authority cannot establish a unique deployment-wide coordinator. |

The broad authority rule remains a warning because local state may be an optimization backed by a durable source.
`process-local-coordination` requires explicit high-confidence provider evidence and deployment-wide required scope.
The narrower named mechanisms should be added only when real examples justify distinct facts or messages.

## Consistency

Consistency policies are deferred until there is evidence for both sides of the boundary:

- a deployment fact declaring an explicit consistency guarantee, such as eventual visibility; and
- a code fact showing that a correctness decision relies on stronger read semantics.

No consistency policy should be added from a database client call or product name alone. The architectural consequence
must follow from the joined guarantees.

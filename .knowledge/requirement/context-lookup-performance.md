---
id: requirement:context-lookup-performance
type: requirement
title: Bounded Context Value Lookup
---
Framework request state resolves in one context.Value lookup, unavoidably nested values reach ancestors by parent pointer, and TinyBind generated code reads framework state through resolvers instead of its own context nodes.

```yaml
status: implemented
proposed: 2026-08-06
implemented: 2026-08-06
source: user request 2026-08-06
why: context.Value walks the chain outward, so cost is O(depth) per lookup and one type assertion plus binary size per extra node; upstream measured about 1.2 ns per level and collapsed any depth to the depth-1 number with one bundle node, 2026-08-05
asks:
  flat_values:
    what: one capsule per request holding every stable framework resource
    status: already the contract of policy:context-value-storage and data:request-context-capsule; this requirement added nothing there
  nested_values:
    which: values whose parent-child shadowing is semantic — the active span each contrib/otel Tracer.Start creates, and the derived capsule copies pwruntime installs
    ask: each stored value carries a pointer to its parent value, so one context.Value fetch reaches the innermost and every ancestor is a pointer chase outside the context chain
    span: contrib/otel/trace Span gained a parent field fixed at Start, Parent and Root accessors, and SpanFromContext returning the innermost local span; a remote extracted parent has no local Span, so its child's parent is nil
    capsule: every derived pwruntime Resources copy records the Resources it came from, exposed as Parent, nil at the request root
    transaction: needed no pointer of its own — one TransactionScope serves a whole transaction and savepoint nesting is a depth counter on that one value, so ancestry never touches the context
  generated_code:
    ask: TinyBind runtime entries stop calling context.Value on their own private nodes inside every operation
    sql: decision:tinybind-sql-runtime already consumed the configured SQLExecutorResolver; pwruntime withScope additionally stopped installing the sqlbind executor node beside the capsule, leaving that key an input seam for an externally opened transaction only
    nosql:
      built: api:dynamo-package and api:firestore-package each own one process handle built at setup with NewHandle, exposed as Handle(ctx); generated queries reach it through the DynamoHandleResolver and FirestoreHandleResolver generation options, and handler code uses the parameter form ("On"-suffixed entries)
      middleware_removed: neither package installs a per-request context node any more; setup returns no middleware
      fallback: Handle honours a handle installed with WithClient or WithHandle when the process holds no client, which is what a unit test building a bare context uses
      framework_stores: the sessionstore and authstate dynamo and firestore stores resolve through the same Handle functions instead of TableFromContext
      scaffold: the pw init dynamo record scaffold demonstrates the Handle plus On-form pattern
  handle_placement_correction:
    proposed_was: the capsule carries the dynamobind Handle
    is: process state owned by each database package, because pwruntime importing dynamobind would link the DynamoDB driver into every binary — the same import-boundary reason TinyBind could not own the bundle struct upstream
    consequence: the common path reads no context at all; the client is a process deployment fact, so per-request carriage was never needed
  pin: system:tinybind moved to v0.4.1, which ships the resolver options, the On entries, NewHandle, and HandleFromContext
acceptance:
  - a request-path framework resource read costs at most one context.Value walk plus one type assertion; the NoSQL handle path costs none
  - root and ancestor span access from a nested span performs no context traversal
  - a savepoint-nested transaction reaches its ancestor state without context traversal
  - no TinyBind-owned context node is installed by the framework into a request context
  - the api:request-context-accessors surface is unchanged; only resolution paths moved
non_goals:
  - removing context.Context parameters; cancellation and deadline stay context mechanisms and every driver entry keeps its leading context
  - a request-latency claim; per upstream measurement the win is node count, assertion count, and binary size
  - exposing the capsule or parent pointers as application API; Span.Parent and Resources.Parent are framework-side accessors
breaking:
  - application code calling context-form dynamobind or firestorebind entries must move to the On form with the package Handle, because no middleware installs a client node any more
  - EnsureClient remains for code handing a context to something still calling context-form entries
```

---
id: system:tinygodriver-dynamodb
type: system
title: tinygodriver DynamoDB Client
---
The TinyGo-buildable DynamoDB client Popcorn Wave configures and hands to generated code; it speaks the DynamoDB JSON protocol directly instead of through aws-sdk-go-v2.

```yaml
package: github.com/shibukawa/tinygodriver/nosql/dynamodb
part_of: system:tinygodriver
minimum_version: v1.1.3, the release that introduced nosql/dynamodb
upstream_catalog: the tinygodriver repository ships its own concepts for the client, attribute value, retry policy, connection policy, JSON codec, and local endpoint; read them there rather than restating them
recorded_here: only the facts a Popcorn Wave concept depends on
constructor: New(opts ...Option) (*Client, error)
credentials:
  default: environment, through WithCredentialsFromEnv
  explicit: WithCredentials
  region: environment, or WithRegion; absent is an error rather than a default
  endpoint: AWS_ENDPOINT_URL_DYNAMODB, then AWS_ENDPOINT_URL, then the regional host; WithEndpoint overrides
table_admin:
  present: [CreateTable, DeleteTable, DescribeTable, ListTables]
  absent: UpdateTable, which the driver places out of scope
  consequence: requirement:dynamodb-migration can create a table and read one back, and cannot alter one
  waiting: DescribeTable is exposed and polling it is the caller's loop; the driver ships no waiter
schema_types:
  TableDefinition: "{Name; PartitionKey KeyAttribute; SortKey *KeyAttribute; BillingMode; ReadCapacity; WriteCapacity; GlobalIndexes; LocalIndexes}"
  KeyAttribute: "{Name string; Type AttributeType}" with S, N or B only
  BillingMode: PayPerRequest default, Provisioned
  TableDescription: what DescribeTable returns; the observed side of the same shape
errors:
  wrapper: "*dynamodb.Error with Op, Table, StatusCode, RequestID, Unwrap, Retryable"
  sentinels_used_by_pw: [ErrTableNotFound, ErrResourceNotFound, ErrTableInUse, ErrValidation, ErrBadCredentials, ErrNoRegion]
  passthrough: system:tinybind dynamobind wraps with %w or not at all, so a Popcorn Wave caller matches these directly
transport:
  http_client: WithHTTPClient replaces the whole client, which is the seam decision:dynamodb-observability-seam uses
  close: Close releases pooled TLS handles, and is skipped for a client built with WithHTTPClient because its owner may still use it
  pool: 4 idle connections per host by default, raised with WithMaxIdleConns; one host per client
excluded_by_the_driver: [transactions, PartiQL, Streams, DAX, TTL administration, autoscaling, tags, global tables, backup]
transactions:
  answer: not supported and not reachable; the driver declares no TransactWriteItems or TransactGetItems
  misleading_sentinel: ErrTransactionConflict exists and is not a trace of support; DynamoDB returns it to an ordinary PutItem whose item is held by someone else's transaction, and the driver maps it as retryable
  effect_here: api:transaction-runner has no DynamoDB meaning, and no future one is planned
upstream_requests:
  ranked: 2026-07-31 by how much one driver change unlocks downstream, per decision:dynamodb-framework-scope
  UpdateTimeToLive:
    status: withdrawn from this side on 2026-08-01, per decision:dynamodb-operational-configuration
    was: ranked first, because it appeared to unlock the session backend
    why_withdrawn: the framework would not call it if it had it; a deployed table's TTL is defined by deployment tooling
    unaffected: system:tinybind still wants its own TTL tag, which declares an attribute rather than applying a setting
  UpdateTable:
    priority: second
    unlocks: adding a secondary index to a live table, without which an index-bearing table can only be created and never evolved
    note: the same call site as the TTL work, so the two are worth sending together
  transactions:
    priority: third, and larger than the other two combined
local_server: amazon/dynamodb-local over http, which is what api:cli-dev starts for a scaffolded project
```

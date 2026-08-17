---
title: DynamoDB Query Format
description: The dynamo struct tag, the .pw.dynamo declaration grammar, and every check generation runs against them.
sidebar:
  order: 4
---

DynamoDB access has two declarations, and both are checked against each other.
A Go struct carrying `dynamo` tags *is* the schema — there is no separate DDL —
and a `.pw.dynamo` file declares the access patterns that read it. `pw generate`
turns the first into an item codec and a table definition, and the second into
one named function per pattern.

This page is both surfaces. For turning the store on, the `[middleware.dynamo]`
keys, and how schema is applied, see [DynamoDB](/guides/storage/dynamodb/).

## Where generation looks

Both live in a directory listed under `generate.dynamo` in `popcornwave.toml`,
which `pw add dynamo` writes. A `.pw.dynamo` outside every listed directory is
reported by path rather than silently skipped.

Generation is directed by use: the codec appears for the directions something
actually calls, so deleting a read shrinks the generated code to match. A
`.pw.dynamo` declaration counts as a use of its result type, so a package whose
only DynamoDB use is a declaration still gets the decoder its query needs.

The key builder is the exception. A type declaring a `partitionkey` receives
`ItemKey` and its table definition whether or not a call needs them, because the
documented way to read an item is `Load(ctx, table, v.ItemKey())` and a method
call is not something generation can discover.

## The `dynamo` tag

```go
type Reading struct {
	Sensor  Sensor    `dynamo:"sensor,partitionkey"`
	At      int64     `dynamo:"at,sortkey"`
	Celsius float64   `dynamo:"celsius"`
	Flags   []string  `dynamo:"flags,stringset,omitempty"`
	Taken   time.Time `dynamo:"taken,unixtime"`
	Ignored string    `dynamo:"-"`
}
```

```
dynamo:"<attribute name>[,<option>...]"
```

An empty name uses the Go field name. `dynamo:"-"` skips the field, and
unexported fields are always skipped.

| Option | Meaning |
| --- | --- |
| `partitionkey` | this field is the table partition key |
| `sortkey` | this field is the table sort key |
| `omitempty` | write no attribute at all when the field is its zero value |
| `stringset` | store a slice as `SS` rather than `L` |
| `numberset` | store a slice as `NS` |
| `binaryset` | store a slice as `BS` |
| `unixtime` | store a `time.Time` as `N` seconds since the epoch |

An option this list does not contain is a generation error, which is the
difference worth having: a reflection-based mapper reads an unknown option as
nothing and quietly stores an `L` where you asked for a set.

The tag is spelled `dynamo`, not the AWS SDK's `dynamodbav`. A field carrying
`dynamodbav` and no `dynamo` is a generation error rather than a field silently
stored under its Go name.

### Attribute types

| Go type | Attribute | Note |
| --- | --- | --- |
| `string` | `S` | the empty string is a value, and is stored |
| `int`…`int64`, `uint`…`uint64` | `N` | via `strconv`, never through `float64` |
| `float32`, `float64` | `N` | |
| `bool` | `BOOL` | |
| `[]byte` | `B` | |
| `time.Time` | `S` as RFC 3339 nano, or `N` with `unixtime` | |
| `[]T` | `L`, or `SS`/`NS`/`BS` with a set option | |
| `map[string]T` | `M` | a non-string key is a generation error |
| nested struct | `M` | must be declared in the same package |
| `*T` | the pointee, or `NULL` when nil | |
| `dynamodb.AttributeValue` | stored as it stands | the escape hatch |

A named type works wherever its underlying type does, so `type Sensor string` is
an `S` and the generated code converts.

Numbers are text from end to end. A DynamoDB number carries 38 significant
digits and `float64` does not, so nothing here routes one through a float, and a
value wider than the field is a decode error rather than a silent wrap. A number
with more digits than any Go type holds still round-trips through a
`dynamodb.AttributeValue` field.

Decoding leaves a field alone when the item carries no such attribute, so an
item written by an older version of the struct decodes without error.

## Query declarations

```
[export] statement <Name>(<param>: <GoType>, ...): dynamo.<shape><<ItemType>> {
  table <name>
  key <attribute> = {param} [and <attribute> <predicate>]
}
```

```
export statement ReadingsSince(sensor: Sensor, from: int64): dynamo.many<Reading> {
  table reading
  key sensor = {sensor} and at > {from}
}

export statement ReadingsBetween(sensor: Sensor, lo: int64, hi: int64): dynamo.page<Reading> {
  table reading
  key sensor = {sensor} and at between {lo} and {hi}
}

statement readingsForSensor(sensor: Sensor): dynamo.many<Reading> {
  table reading; key sensor = {sensor}
}
```

- Parameter types are Go types as your package spells them, including named
  types and `[]byte`.
- Both clauses are required. They may appear in either order, and `;` separates
  them on one line.
- `//` starts a comment to end of line.
- `export` has to agree with the name's own casing, since Go decides visibility
  by the name. Either one without the other is a generation error rather than a
  silent rename.

### Result shapes

The shape picks the *request* shape rather than a row count, since a Query
always returns many:

| Shape | Generated return | Requests |
| --- | --- | --- |
| `dynamo.many<T>` | `iter.Seq2[T, error]` | one per page, as the range advances |
| `dynamo.page<T>` | `(dynamobind.Page[T], error)` | exactly one |

`Page[T]` carries `Count`, `ScannedCount`, and `LastEvaluatedKey`. An iterator
reports none of those, so a query whose filter scans a hundred times what it
returns looks exactly like one that does not, and an interrupted run cannot be
resumed. How many requests to make stays the author's decision, which is why
both exist.

### Key conditions

The partition key predicate is mandatory, comes first, and is always `=`,
because DynamoDB allows nothing else there. At most one sort key predicate may
follow:

| Written | Sends |
| --- | --- |
| `at = {p}` | `=` |
| `at < {p}`, `at <= {p}`, `at > {p}`, `at >= {p}` | the comparison |
| `at between {lo} and {hi}` | `BETWEEN` |
| `begins_with(at, {p})` | `begins_with`, on a string sort key only |

### The `table` clause

`table` names the table this pattern runs against, which is what removes the
table parameter from the generated signature. It sits in the body rather than on
the type because a type is not one table: the same struct is stored in a test
table and a production one, so a table on the type would assert something
untrue.

The name is checked against what DynamoDB accepts — three to 255 characters of
letters, digits, `_`, `-`, and `.` — so a name the service would reject is a
generation error rather than a `ValidationException` on the first call.

A deployment that names the table differently is not written here; see
[declared and deployed names](#declared-and-deployed-names).

### Generated signature

```go
func ReadingsSince(ctx context.Context,
	sensor Sensor, from int64, opts ...dynamodb.QueryOption) iter.Seq2[Reading, error]
```

There is no table parameter and no client parameter. The variadic options reach
the driver, so `dynamodb.WithLimit`, `WithScanForward`, `WithConsistentRead`, and
`WithIndex` all work; the generated expression names and values are appended
last, so a caller option cannot replace the condition the declaration describes.

### Reserved words are handled for you

DynamoDB reserves 573 words, including `status`, `name`, `size`, `type`, `data`,
`year`, `count`, and `timestamp`, and an expression naming one literally is
rejected with `ValidationException`. Generated queries alias every attribute
unconditionally, so the question never arises:

```go
const readingsSinceKeyCondition = "#k0 = :v0 AND #k1 > :v1"

var readingsSinceAttributeNames = map[string]string{"#k0": "sensor", "#k1": "at"}
```

Because the names are known at generation time, the expression and the alias map
are constants: nothing is assembled per call, and no reserved-word list has to be
carried or kept current.

## Item operations

```go
h, err := dynamo.Handle(ctx)

LoadOn[T](ctx, h, table, key, opts...) (T, error)
StoreOn(ctx, h, table, v, opts...) error
RemoveOn(ctx, h, table, v, opts...) error
UpdateOn(ctx, h, table, v, expression, opts...) error

StoreReturningOn(ctx, h, table, v, opts...) (T, bool, error)
RemoveReturningOn(ctx, h, table, v, opts...) (T, bool, error)

QueryPageOn[T](ctx, h, table, keyCond, opts...) (Page[T], error)
ScanPageOn[T](ctx, h, table, opts...) (Page[T], error)
QueryOn[T](ctx, h, table, keyCond, opts...) iter.Seq2[T, error]
ScanOn[T](ctx, h, table, opts...) iter.Seq2[T, error]

StoreAllOn(ctx, h, table, vs) (unprocessed []T, err error)
LoadAllOn[T](ctx, h, table, keys, opts...) (items []T, unprocessed []dynamodb.Key, err error)
```

These take a table name because they have no declaration to read one from. The
handle is `database/dynamo`'s process client bound to the configured table
naming; a call that runs with the section disabled gets a named no-client error
rather than a panic. A declared query resolves the same handle itself, which is
why only these direct entries take it.

`Store` is `PutItem` and replaces the whole item. `Update` takes a DynamoDB
update expression verbatim and supplies only the key, which is the part a struct
tag can actually provide. `StoreReturning` and `RemoveReturning` ask for
`ALL_OLD`; their bool is false when there was nothing there, which is not an
error.

`StoreAll` and `LoadAll` split the input into requests DynamoDB accepts —
`MaxBatchWrite` is 25 and `MaxBatchGet` is 100, both exported so a caller sizing
its own input reads the same numbers the chunking uses. What the service
declined comes back as `unprocessed`; the retry policy is yours, because the
driver has already retried the transport and a loop here would multiply that
silently. `LoadAll` returns items in whatever order DynamoDB replies with, and a
key matching nothing is simply absent.

The unchecked string key-condition form of `Query` and `QueryPage` stays
available for what a declaration cannot express. Nothing checks that text
against your tags, and the reserved words above are yours to alias.

## Declared and deployed names

The name in a `table` clause and the name an item operation passes are what your
code declares. One function maps them onto the deployed names, and every runtime
entry — the request path and the migrator alike — runs it:

```toml
[middleware.dynamo]
table_prefix = "myapp-"

[[middleware.dynamo.table_names]]
declared = "reading"
deployed = "readings-prod-8f21c"
```

An explicit entry wins; otherwise the prefix applies; otherwise the declared
name stands. `dynamo.WithTableResolver` replaces the composed function outright
for a deployment neither key expresses. A `table_names` entry naming a table no
code declares is an error rather than a line that silently does nothing.

## Errors

Every driver sentinel survives, so a miss stays a miss rather than arriving as a
zero value:

```go
_, err := dynamobind.Load[Reading](ctx, "reading", key)
if errors.Is(err, dynamodb.ErrItemNotFound) { … }

var driverError *dynamodb.Error
if errors.As(err, &driverError) {
	log.Println(driverError.Op, driverError.RequestID, driverError.Retryable())
}
```

A decode failure names the attribute and both kinds. `AsError` walks the chain
without `errors.As`, which needs reflection:

```go
if mapping, ok := dynamobind.AsError(err); ok {
	log.Println(mapping.Attribute, mapping.Expected, mapping.Got) // at N S
}
```

## Generation errors

Every check names the type and the field, or the statement and the attribute.

Tag and type checks:

- an unknown `dynamo` tag option
- a `dynamodbav` tag on a field with no `dynamo` tag
- two fields mapping to one attribute name
- two `partitionkey` fields, two `sortkey` fields, or a `sortkey` without a
  `partitionkey`
- a key field whose attribute is not `S`, `N`, or `B`
- a Go type with no attribute form, a map with a non-string key, or a set option
  whose element type does not match
- a nested struct declared in another package
- a type that already declares `EncodeItem`, `DecodeItem`, or `ItemKey` by hand

Query checks:

- a statement with no `table` clause, or with two
- a table name DynamoDB would reject
- an item type with no `dynamo` tags, or one with no `partitionkey`
- an attribute the type does not have
- a non-key attribute in the key clause — the message names the clause it
  belongs in
- a partition key predicate that is not `=`, or one that is not first
- more than one sort key predicate
- `begins_with` on an attribute that is not stored as a string
- a parameter whose type does not match the attribute's Go type
- a placeholder naming no declared parameter, or a parameter never used
- two statements with one name

## Outside the declarable surface

| Absent | Consequence |
| --- | --- |
| Filter, projection, condition, and update expressions | a `filter` clause is rejected with a message saying so; pass those expressions yourself |
| Secondary indexes | there is no `gsi` tag, so a declared query runs against the table's own keys; `dynamodb.WithIndex` still reaches the driver, but nothing checks the condition against that index |
| Single-table design | one struct owns one table, since `<Type>Table` describes one type and a typed read decodes every item as one type |
| Optimistic locking and TTL | there is no `version` tag and no `ttl` tag; both are the caller's to manage |
| Transactions, PartiQL, Streams, DAX | the driver excludes them |

None of these is a Popcorn Wave choice. They are the edge of what the layer
below does.

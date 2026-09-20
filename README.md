# mina-archive-sdk-go

[![CI](https://github.com/o1-labs/mina-archive-sdk-go/actions/workflows/ci.yml/badge.svg)](https://github.com/o1-labs/mina-archive-sdk-go/actions/workflows/ci.yml)
[![Go reference](https://pkg.go.dev/badge/github.com/o1-labs/mina-archive-sdk-go.svg)](https://pkg.go.dev/github.com/o1-labs/mina-archive-sdk-go)
[![release](https://img.shields.io/github/v/tag/o1-labs/mina-archive-sdk-go?label=release&sort=semver&logo=go)](https://github.com/o1-labs/mina-archive-sdk-go/releases)
[![license](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](./LICENSE)

Go SDK for [Mina Protocol's Archive Node](https://github.com/o1-labs/Archive-Node-API) GraphQL endpoint.

Companion to the daemon-targeting [`MinaProtocol/mina-sdk-go`](https://github.com/MinaProtocol/mina-sdk-go) / [`mina-sdk-rust`](https://github.com/MinaProtocol/mina-sdk-rust) / [`mina-sdk-python`](https://github.com/MinaProtocol/mina-sdk-python). This SDK targets the separate **archive** endpoint defined by `o1-labs/Archive-Node-API` (events, actions, blocks, network state).

## Install

```sh
go get github.com/o1-labs/mina-archive-sdk-go/v2@latest
```

Requires Go ≥ 1.21.

## Quick start

```go
package main

import (
    "context"
    "fmt"

    archive "github.com/o1-labs/mina-archive-sdk-go/v2"
)

func main() {
    client := archive.NewClient(archive.WithGraphQLURI("https://archive.example/"))
    defer client.Close()

    events, err := client.GetEvents(context.Background(), archive.EventFilterOptionsInput{
        Address: "B62q...",
        Status:  archive.BlockStatusCanonical,
        From:    archive.IntPtr(100),
        To:      archive.IntPtr(200),
    })
    if err != nil {
        panic(err)
    }
    fmt.Printf("got %d event group(s)\n", len(events))
}
```

> **The endpoint is the root path.** Archive-Node-API serves GraphQL at `/`, not
> `/graphql`. Pass the base URL as-is — the SDK never appends a path, so a URL
> ending in `/graphql` reaches a route the server does not serve and returns 404.

## API

Each method on `*Client` maps 1:1 to a GraphQL query in the [Archive-Node-API schema](./schema.graphql):

| Method | Returns | Description |
| --- | --- | --- |
| `GetEvents(ctx, input)` | `[]*EventOutput` | Events emitted by a zkApp account, optionally filtered by block range and consensus status. |
| `GetActions(ctx, input)` | `[]*ActionOutput` | Actions dispatched from a zkApp account. |
| `GetNetworkState(ctx)` | `*NetworkStateOutput` | Archive's max canonical / pending block heights. |
| `GetBlocks(ctx, opts)` | `[]*Block` | Blocks filtered by height/date range and chain status. Transaction detail needs `ENABLE_BLOCK_TRANSACTION_DETAILS` on the server — see below. |
| `GetVerificationKeyUpdates(ctx, in)` | `[]VerificationKeyUpdate` | Applied account updates that set a given verification key, within a required block range. |
| `ExecuteQuery(ctx, gql, vars, name)` | `json.RawMessage` | Low-level escape hatch returning the raw `data` field. |

### Block transaction detail

`GetBlocks` returns transaction detail only when the server sets
`ENABLE_BLOCK_TRANSACTION_DETAILS=true`. It **defaults to `false`**, and on a stock
server every block comes back with `ParentHash` as `""` and `UserCommands`,
`ZkappCommands` and `FeeTransfer` all empty. `Coinbase` **is** populated either way,
which is what makes the response look healthy rather than obviously truncated.

### Configuration

```go
client := archive.NewClient(
    archive.WithGraphQLURI("https://archive.example/"),
    archive.WithRetries(5),
    archive.WithRetryDelay(10*time.Second),
    archive.WithTimeout(60*time.Second),
    archive.WithHeader("X-Api-Key", os.Getenv("API_KEY")),
)
```

### Dates and times

The schema carries **two different time encodings**, a few fields apart, and both
arrive as strings:

| Field | Encoding | Example |
| --- | --- | --- |
| `BlockInfo.Timestamp` | Unix epoch **milliseconds**, decimal string | `"1692054601000"` |
| `Block.DateTime` | ISO-8601 | `"2023-08-14T23:10:01.000Z"` |

`BlockInfo.Timestamp` is a raw pass-through of the archive DB column. `time.Parse(time.RFC3339, ...)`
fails on it, and reading it as seconds puts the block in the wrong century. Each type
has a `Time()` accessor that handles its own encoding:

```go
t, err := blockInfo.Time() // parses Unix epoch milliseconds
t, err := block.Time()     // parses ISO-8601
```

On input, `DateTimeGte` / `DateTimeLt` must be ISO-8601. The server coerces them with
JavaScript's `new Date(value).getTime()`, and a value it cannot parse becomes `NaN`,
which reaches SQL as the string `"NaN"` and **matches nothing without erroring** —
HTTP 200, empty list, no diagnostic anywhere:

```text
"2023-08-14T00:00:00Z"  -> 1691971200000    ok
"2023-08-14"            -> 1691971200000    ok
"14/08/2023"            -> NaN              silently returns zero rows
"Aug 14 2023"           -> 1691964000000    parses, but timezone-dependent
```

Build the bounds from `time.Time` instead of formatting them by hand:

```go
var q archive.BlockQueryInput
q.SetDateTimeRange(from, to) // lower inclusive, upper exclusive
```

### Currency helper

`Currency` wraps nanomina amounts in a `uint64` for safe parsing of coinbase / fee / user-command values:

```go
coinbase, err := archive.CurrencyFromGraphQL(block.Transactions.Coinbase)
if err != nil {
    return err
}
fmt.Printf("coinbase: %s MINA\n", coinbase)  // "720.000000000 MINA"

fee := archive.MustCurrencyFromMina("0.01")
total := coinbase.Add(fee)
```

### Error handling

Every error a query can return is one of the types below, and the list is
exhaustive — no query path returns a bare `fmt.Errorf` value.

| Type | Means | Retried? |
| --- | --- | --- |
| `*InvalidInputError` | The client rejected the request before sending it — a GraphQL `Int` out of range, or variables that will not marshal. **No HTTP request is made.** | — |
| `*RateLimitError` | HTTP 429. `RetryAfter` says how long to wait. | yes |
| `*GraphQLError` | A populated `errors` array. Deterministic, so not retried. `Data` may hold a partial payload. | no |
| `*HTTPError` | A non-2xx that is not GraphQL-shaped — usually a wrong URL path. `Body` is the raw response, truncated. | no |
| `*DecodeError` | HTTP 200 whose body is not a readable GraphQL response, or whose `data` does not fit the typed result. | no |
| `*MissingFieldError` | A well-formed envelope missing the field the query asked for — including a bare `{}`, which has no `data` at all. | no |
| `*ConnectionError` | Retries exhausted against transport/5xx failures, **or** the context was cancelled during backoff. `Unwrap` exposes the cause, so `errors.Is(err, context.DeadlineExceeded)` works. | exhausted |

```go
import "errors"

_, err := client.GetEvents(ctx, ...)
var (
    invErr  *archive.InvalidInputError
    rateErr *archive.RateLimitError
    gqlErr  *archive.GraphQLError
    httpErr *archive.HTTPError
    decErr  *archive.DecodeError
    missErr *archive.MissingFieldError
    connErr *archive.ConnectionError
)
switch {
case errors.As(err, &invErr):
    // Rejected locally; nothing was sent. invErr.Field names the offender.
case errors.As(err, &rateErr):
    // Rate limited. rateErr.RetryAfter says how long to wait.
case errors.As(err, &gqlErr):
    // Server-side validation, malformed query, etc. Not retried.
case errors.As(err, &httpErr):
    // A non-2xx that is not GraphQL-shaped — often a wrong URL path.
case errors.As(err, &decErr):
    // HTTP 200 that is not a GraphQL response. decErr.Body shows what came back.
case errors.As(err, &missErr):
    // Server returned an unexpected shape — likely a schema mismatch.
case errors.As(err, &connErr):
    // Exhausted retries, or cancelled mid-backoff. Inspect connErr.LastError.
}
```

`Currency` adds `*InvalidCurrencyError` and `*CurrencyUnderflowError`, which no
query path returns.

#### Contract error codes

Branch on `extensions.code`, never on message text — the server blocks GraphQL field
suggestions, so messages are deliberately minimal and carry no stability promise.

| Constant | Code | Meaning |
| --- | --- | --- |
| `archive.CodeBlockRangeError` | `BLOCK_RANGE_ERROR` | Range exceeds `BLOCK_RANGE_SIZE`. Narrow it; never retry unchanged. |
| `archive.CodeActionStateNotFound` | `ACTION_STATE_NOT_FOUND` | The action state is not in the archive. |
| `archive.CodeActionStateOutOfRange` | `ACTION_STATE_OUT_OF_RANGE` | The action state is outside the requested range. |
| `archive.CodeRateLimited` | `RATE_LIMITED` | Too many requests. Back off. |

```go
var gqlErr *archive.GraphQLError
if errors.As(err, &gqlErr) && gqlErr.HasCode(archive.CodeBlockRangeError) {
    // Halve the range and try again.
}
```

All of these arrive as **HTTP 200** with a populated `errors` array;
`extensions["status"]` is a payload field, not the HTTP status. An empty `Code()`
means no code was sent — the server masks unexpected errors, and those carry no
`extensions` at all — not that nothing went wrong.

#### Rate limiting

Every GraphQL-level error from this API arrives as **HTTP 200** with a populated
`errors` array, so HTTP 429 is the only non-200 it emits under normal operation. That
makes it unusually informative: it unambiguously means "slow down", and it is the one
case where retrying the identical request is correct.

The client retries 429 automatically, waiting the interval named in `Retry-After`. If
the retries run out it returns `*RateLimitError`, which carries `RetryAfter`, `Limit`
and `Remaining` so a caller can schedule its own back-off. A 429 is never reported as a
`*GraphQLError`.

`RetryAfter` is zero when the server sent no usable header, and `Limit`/`Remaining` are
`-1` when theirs were absent or malformed — so a missing header is distinguishable from
a real zero.

#### Partial results

A response can legally carry **both** `data` and `errors`. The root lists and most of
their fields are nullable, so a field-level resolver error nullifies a sub-tree rather
than the whole response, and the rows that succeeded still arrive.

`GetEvents` and friends still return an error in that case — a partial result is not a
success — but the payload is attached to `*GraphQLError.Data` instead of being thrown
away. On a long block-range walk this is the difference between losing one row and
losing the whole page:

```go
var gqlErr *archive.GraphQLError
if errors.As(err, &gqlErr) && gqlErr.HasPartialData() {
    var payload struct {
        Events []archive.EventOutput `json:"events"`
    }
    if err := json.Unmarshal(gqlErr.Data, &payload); err == nil {
        // payload.Events holds the rows the server did return.
    }
}
```

`Data` is nil when the server sent `"data": null`, which is a total failure rather than
a partial one — `HasPartialData` tells the two apart.

## Examples

```sh
ARCHIVE_GRAPHQL_URI=https://archive.example/ go run ./examples/networkstate
```

See `examples/`:

- `events/` — query events for an address
- `actions/` — query actions for an address
- `blocks/` — get the latest canonical blocks with currency parsing
- `networkstate/` — check archive sync state

## Nullable elements

`GetEvents`, `GetActions` and `GetBlocks` return `[]*T`. The SDL types these
`[T]!`: the list itself is always present, but **every element is nullable**,
and the server is free to return `null` there indefinitely — under the upstream
versioning policy `T` → `T!` is the only safe direction, so a null element
never becomes a breaking change.

The same holds for `EventData.Data`, `ActionData.Data` (`[]*string`),
`TransactionInfo.ZkappAccountUpdateIDs` (`[]*int`) and both `FailureReason`
fields (`*string`). A value type in those positions decoded a `null` to a zero
value indistinguishable from real data: a null `[Block]` element became a block
at height 0 with empty hashes, a null id became id 0, and
`FailureReason == ""` could not tell "did not fail" from "failed with an empty
reason".

`GetVerificationKeyUpdates` is the exception: its SDL type is
`[VerificationKeyUpdate!]!`, elements included, so it returns values.

## Version compatibility

The module exports the schema version it speaks:

```go
archive.SchemaVersion // "1.0" — the Archive-Node-API schema major.minor
```

**`SchemaVersion`, not the module version, is the compatibility check.** The
module version is plain semver about the SDK's own surface:

| Part | Meaning |
| --- | --- |
| **Major** | A breaking change to the SDK's API — whether the schema forced it or not. |
| **Minor** | Additive: a new query, a new option, a new helper. |
| **Patch** | Fixes, docs, dependencies. |

The two still move together in the common cases: a breaking schema change
breaks the SDK surface, so it takes a major, and a schema minor that adds a
query is an SDK minor. What separates them is an **SDK-only** breaking change,
which now has a home. v2 is exactly that — it moved six positions to pointer
form so the `null`s the 1.0 schema always permitted stop decoding to zero
values, and it speaks the same `1.0` schema v1 did.

The schema is additive within a major version, so a module whose
`SchemaVersion` major matches the server keeps working against a newer server;
it simply cannot reach what was added after it.

Earlier releases followed a stricter rule in which the module's major.minor
*was* the schema version. That rule left no position for a breaking SDK-only
fix, which is why it was amended in v2.

### Migrating from v1 to v2

The import path carries the major, as Go requires:

```go
archive "github.com/o1-labs/mina-archive-sdk-go/v2"
```

`GetEvents`, `GetActions` and `GetBlocks` now return `[]*T`, and six positions
moved to pointer form. Field access through a non-nil element is unchanged —
Go dereferences transparently — so the compiler will only stop you where you
compare or copy a value:

```go
// v1 — a null element silently became a Block at height 0
for _, b := range blocks {
    fmt.Println(b.BlockHeight)
}

// v2 — the nil is visible
for _, b := range blocks {
    if b == nil {
        continue
    }
    fmt.Println(b.BlockHeight)
}
```

`FailureReason` changes from `string` to `*string`, so `cmd.FailureReason != ""`
becomes `cmd.FailureReason != nil`. `Data` and `ZkappAccountUpdateIDs` members
need a dereference.

`GetNetworkState` and `GetVerificationKeyUpdates` are unchanged —
`[VerificationKeyUpdate!]!` has non-nullable elements and was already correct.

## Development

```sh
go build ./...
go test ./...           # unit tests, no infra needed
go vet ./...
```

Integration tests run against a live Archive-Node-API instance. CI does this in `.github/workflows/integration.yml`. To run locally:

```sh
ARCHIVE_GRAPHQL_URI=http://localhost:8080/ go test -run TestIntegration ./...
```

## Schema sync

`schema.graphql` is vendored from `o1-labs/Archive-Node-API@main`. The `Schema Drift` CI workflow compares them weekly and on PR; on drift, update both `schema.graphql` and `types.go` in the same PR.

## License

Apache-2.0 — see [`LICENSE`](./LICENSE).

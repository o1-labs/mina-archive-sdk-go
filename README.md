# mina-archive-sdk-go

[![CI](https://github.com/o1-labs/mina-archive-sdk-go/actions/workflows/ci.yml/badge.svg)](https://github.com/o1-labs/mina-archive-sdk-go/actions/workflows/ci.yml)
[![Go reference](https://pkg.go.dev/badge/github.com/o1-labs/mina-archive-sdk-go.svg)](https://pkg.go.dev/github.com/o1-labs/mina-archive-sdk-go)
[![release](https://img.shields.io/github/v/tag/o1-labs/mina-archive-sdk-go?label=release&sort=semver&logo=go)](https://github.com/o1-labs/mina-archive-sdk-go/releases)
[![license](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](./LICENSE)

Go SDK for [Mina Protocol's Archive Node](https://github.com/o1-labs/Archive-Node-API) GraphQL endpoint.

Companion to the daemon-targeting [`MinaProtocol/mina-sdk-go`](https://github.com/MinaProtocol/mina-sdk-go) / [`mina-sdk-rust`](https://github.com/MinaProtocol/mina-sdk-rust) / [`mina-sdk-python`](https://github.com/MinaProtocol/mina-sdk-python). This SDK targets the separate **archive** endpoint defined by `o1-labs/Archive-Node-API` (events, actions, blocks, network state).

## Install

```sh
go get github.com/o1-labs/mina-archive-sdk-go@latest
```

Requires Go ≥ 1.21.

## Quick start

```go
package main

import (
    "context"
    "fmt"

    archive "github.com/o1-labs/mina-archive-sdk-go"
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
| `GetEvents(ctx, input)` | `[]EventOutput` | Events emitted by a zkApp account, optionally filtered by block range and consensus status. |
| `GetActions(ctx, input)` | `[]ActionOutput` | Actions dispatched from a zkApp account. |
| `GetNetworkState(ctx)` | `*NetworkStateOutput` | Archive's max canonical / pending block heights. |
| `GetBlocks(ctx, opts)` | `[]Block` | Blocks filtered by height/date range and chain status. Transaction detail needs `ENABLE_BLOCK_TRANSACTION_DETAILS` on the server — see below. |
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

All public errors implement `error`. Match with `errors.As`:

```go
import "errors"

_, err := client.GetEvents(ctx, ...)
var gqlErr *archive.GraphQLError
var connErr *archive.ConnectionError
var missErr *archive.MissingFieldError
switch {
case errors.As(err, &gqlErr):
    // Server-side validation, malformed query, etc. Not retried.
case errors.As(err, &connErr):
    // Exhausted retries (network / 5xx). Inspect connErr.LastError.
case errors.As(err, &missErr):
    // Server returned an unexpected shape — likely a schema mismatch.
}
```

## Examples

```sh
ARCHIVE_GRAPHQL_URI=https://archive.example/ go run ./examples/networkstate
```

See `examples/`:

- `events/` — query events for an address
- `actions/` — query actions for an address
- `blocks/` — get the latest canonical blocks with currency parsing
- `networkstate/` — check archive sync state

## Version compatibility

This SDK versions in lockstep with the [Archive-Node-API](https://github.com/o1-labs/Archive-Node-API) schema it speaks.

| Part | Meaning |
| --- | --- |
| **Major** | The schema major version. A breaking schema change moves both. |
| **Minor** | The schema minor version. A new query or argument moves both. |
| **Patch** | SDK-only changes — fixes, docs, dependencies. Independent of the server. |

So an SDK on `1.0.x` speaks the `1.0.x` schema, and matching the first two numbers is the whole compatibility check. The schema is additive within a major version, so an older SDK keeps working against a newer server; it simply cannot reach what was added after it.

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

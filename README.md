# mina-archive-sdk-go

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
    client := archive.NewClient(archive.WithGraphQLURI("https://archive.example/graphql"))
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

## API

Each method on `*Client` maps 1:1 to a GraphQL query in the [Archive-Node-API schema](./schema.graphql):

| Method | Returns | Description |
| --- | --- | --- |
| `GetEvents(ctx, input)` | `[]EventOutput` | Events emitted by a zkApp account, optionally filtered by block range and consensus status. |
| `GetActions(ctx, input)` | `[]ActionOutput` | Actions dispatched from a zkApp account. |
| `GetNetworkState(ctx)` | `*NetworkStateOutput` | Archive's max canonical / pending block heights. |
| `GetBlocks(ctx, opts)` | `[]Block` | Blocks filtered by height/date range and chain status, with full transaction detail. |
| `ExecuteQuery(ctx, gql, vars, name)` | `json.RawMessage` | Low-level escape hatch returning the raw `data` field. |

### Configuration

```go
client := archive.NewClient(
    archive.WithGraphQLURI("https://archive.example/graphql"),
    archive.WithRetries(5),
    archive.WithRetryDelay(10*time.Second),
    archive.WithTimeout(60*time.Second),
    archive.WithHeader("X-Api-Key", os.Getenv("API_KEY")),
)
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
ARCHIVE_GRAPHQL_URI=https://archive.example/graphql go run ./examples/networkstate
```

See `examples/`:

- `events/` — query events for an address
- `actions/` — query actions for an address
- `blocks/` — get the latest canonical blocks with currency parsing
- `networkstate/` — check archive sync state

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

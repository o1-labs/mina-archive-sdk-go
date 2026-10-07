// Package archive provides a Go client for interacting with the Mina Archive
// Node GraphQL API.
//
// The archive node indexes Mina blockchain history into a queryable
// PostgreSQL store. This SDK exposes the five queries that the Archive-Node-API
// GraphQL server publishes:
//
//   - Events emitted by a zkApp account, filterable by block range and
//     consensus status.
//   - Actions dispatched from a zkApp account.
//   - Verification-key updates — applied account updates that set a given
//     verification key, within a required block range.
//   - Block details including transactions, by height/date range.
//   - Network state — the archive's max canonical and pending block heights.
//
// Archive-Node-API serves GraphQL at the root path "/", not "/graphql". Pass
// the base URL as-is — the SDK never appends a path, so a URL ending in
// "/graphql" returns 404.
//
// Basic usage — see ExampleClient_GetEvents in example_test.go, which is the
// compiled source of this snippet:
//
//	client := archive.NewClient(archive.WithGraphQLURI("https://archive.example/"))
//	defer client.Close()
//
//	events, err := client.GetEvents(context.Background(), archive.EventFilterOptionsInput{
//	    Address: "B62q...",
//	    Status:  archive.BlockStatusCanonical,
//	})
//
// Companion to MinaProtocol/mina-sdk-go (the Mina daemon GraphQL client) —
// this SDK targets the separate Archive Node GraphQL endpoint defined by
// o1-labs/Archive-Node-API.
//
// # Nullable positions
//
// Schema 2.0 makes the list elements this SDK reads non-null, so the typed
// queries return []T and list members are values. EventOutput.EventData and
// ActionOutput.ActionData stay nullable as a whole: null decodes to a nil
// slice. Both FailureReason fields are nullable scalars (*string): nil means
// the command did not fail.
package archive

// SchemaVersion is the Archive-Node-API schema version this module speaks.
//
// The module major equals the schema major: same major = compatible. A server
// with a higher minor only adds features that this module may not wrap yet.
const SchemaVersion = "2.0"

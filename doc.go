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
// # Nullable elements
//
// GetEvents, GetActions and GetBlocks return []*T. The SDL types these [T]!:
// the list itself is always present, but every element is nullable, and the
// server may keep returning null there indefinitely — under the upstream
// versioning policy T -> T! is the only safe direction, so a null element
// never becomes a breaking change. Guard each element before use. The same
// holds for EventData.Data, ActionData.Data ([]*string),
// TransactionInfo.ZkappAccountUpdateIDs ([]*int) and both FailureReason
// fields (*string), where a value type would decode null to a zero value
// indistinguishable from real data.
//
// GetVerificationKeyUpdates is the exception: its SDL type is
// [VerificationKeyUpdate!]!, elements included, so it returns values.
package archive

// SchemaVersion is the Archive-Node-API schema version this module speaks.
//
// This constant — not the module version — is the compatibility check. The
// module version is plain semver about the SDK's own surface, so an SDK-only
// breaking change can take a major without claiming the schema moved.
//
// The schema is additive within a major version, so a module whose
// SchemaVersion major matches the server keeps working against a newer server;
// it simply cannot reach what was added after it.
const SchemaVersion = "1.0"

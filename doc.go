// Package archive provides a Go client for interacting with the Mina Archive
// Node GraphQL API.
//
// The archive node indexes Mina blockchain history into a queryable
// PostgreSQL store. This SDK exposes the four queries that the Archive-Node-API
// GraphQL server publishes:
//
//   - Events emitted by a zkApp account, filterable by block range and
//     consensus status.
//   - Actions dispatched from a zkApp account.
//   - Block details including transactions, by height/date range.
//   - Network state — the archive's max canonical and pending block heights.
//
// Basic usage:
//
//	client := archive.NewClient(archive.WithGraphQLURI("https://archive.example/graphql"))
//	defer client.Close()
//
//	events, err := client.GetEvents(archive.EventFilterOptionsInput{
//	    Address: "B62q...",
//	    Status:  archive.BlockStatusCanonical,
//	})
//
// Companion to MinaProtocol/mina-sdk-go (the Mina daemon GraphQL client) —
// this SDK targets the separate Archive Node GraphQL endpoint defined by
// o1-labs/Archive-Node-API.
package archive

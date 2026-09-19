// Query archived events for a zkApp account.
//
// Run:
//
//	ARCHIVE_GRAPHQL_URI=http://localhost:8080/ \
//	MINA_ADDRESS=B62q... \
//	  go run ./examples/events
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	archive "github.com/o1-labs/mina-archive-sdk-go/v2"
)

func main() {
	uri := os.Getenv("ARCHIVE_GRAPHQL_URI")
	if uri == "" {
		uri = archive.DefaultGraphQLURI
	}
	addr := os.Getenv("MINA_ADDRESS")
	if addr == "" {
		addr = "B62qiaEMrWiYdK7LcJ2ScdMyG8LzUxi7yaw17XvBD34on7UKfhAkRML"
	}

	client := archive.NewClient(archive.WithGraphQLURI(uri))
	defer client.Close()

	events, err := client.GetEvents(context.Background(), archive.EventFilterOptionsInput{
		Address: addr,
		Status:  archive.BlockStatusCanonical,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("got %d event group(s)\n", len(events))
	limit := 5
	if len(events) < limit {
		limit = len(events)
	}
	// [EventOutput]! has nullable elements: the list is always present, but any
	// element may be nil, so guard each one.
	for _, g := range events[:limit] {
		if g == nil {
			fmt.Println("  (null event group)")
			continue
		}
		height := "?"
		if g.BlockInfo != nil {
			height = fmt.Sprintf("%d", g.BlockInfo.Height)
		}
		fmt.Printf("  block %s: %d event(s)\n", height, len(g.EventData))
	}
}

// Query archived actions for a zkApp account.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	archive "github.com/o1-labs/mina-archive-sdk-go"
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

	actions, err := client.GetActions(context.Background(), archive.ActionFilterOptionsInput{
		Address: addr,
		Status:  archive.BlockStatusCanonical,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("got %d action group(s)\n", len(actions))
	limit := 5
	if len(actions) < limit {
		limit = len(actions)
	}
	for _, g := range actions[:limit] {
		height := "?"
		if g.BlockInfo != nil {
			height = fmt.Sprintf("%d", g.BlockInfo.Height)
		}
		fmt.Printf("  block %s: %d action(s)\n", height, len(g.ActionData))
	}
}

// Check the archive node's sync state — useful for monitoring how far behind
// the archive is relative to the chain.
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

	client := archive.NewClient(archive.WithGraphQLURI(uri))
	defer client.Close()

	state, err := client.GetNetworkState(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	if state.MaxBlockHeight == nil {
		log.Fatal("archive returned no maxBlockHeight — is it synced?")
	}
	m := state.MaxBlockHeight
	fmt.Printf("canonical max: %d\n", m.CanonicalMaxBlockHeight)
	fmt.Printf("pending max:   %d\n", m.PendingMaxBlockHeight)
	fmt.Printf("gap:           %d\n", m.PendingMaxBlockHeight-m.CanonicalMaxBlockHeight)
}

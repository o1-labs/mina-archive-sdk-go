// Query a range of blocks. Demonstrates the Currency helper for parsing
// coinbase / fee amounts that the archive returns as nanomina strings.
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

	canonical := true
	limit := 5
	blocks, err := client.GetBlocks(context.Background(), archive.BlocksOptions{
		Query:  &archive.BlockQueryInput{Canonical: &canonical},
		Limit:  &limit,
		SortBy: archive.BlockSortDesc,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("got %d block(s)\n", len(blocks))
	for _, b := range blocks {
		coinbase, err := archive.CurrencyFromGraphQL(b.Transactions.Coinbase)
		if err != nil {
			fmt.Fprintf(os.Stderr, "block %d: bad coinbase %q: %v\n", b.BlockHeight, b.Transactions.Coinbase, err)
			continue
		}
		fmt.Printf("  block %d by %s…  coinbase=%s MINA  (%d user commands)\n",
			b.BlockHeight, truncatePrefix(b.Creator, 12), coinbase, len(b.Transactions.UserCommands))
	}
}

func truncatePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

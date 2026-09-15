package archive_test

import (
	"context"
	"fmt"

	archive "github.com/o1-labs/mina-archive-sdk-go"
)

// The snippet in doc.go's "Basic usage" block used to be prose, and it did not
// compile — it omitted the ctx argument (#8). Keeping it as a compiled Example
// means the compiler enforces it from now on: if a signature changes and this
// is not updated, `go test ./...` fails.
//
// It is deliberately not run against a live server — there is no Output
// comment, so `go test` compiles it but does not execute it.
func ExampleClient_GetEvents() {
	client := archive.NewClient(archive.WithGraphQLURI("https://archive.example/"))
	defer client.Close()

	events, err := client.GetEvents(context.Background(), archive.EventFilterOptionsInput{
		Address: "B62q...",
		Status:  archive.BlockStatusCanonical,
	})
	if err != nil {
		return
	}
	fmt.Printf("got %d event group(s)\n", len(events))
}

// The five queries the SDK exposes, so the doc.go bullet list cannot drift
// from the real method set without the compiler noticing.
func ExampleClient() {
	client := archive.NewClient(archive.WithGraphQLURI("https://archive.example/"))
	defer client.Close()

	ctx := context.Background()
	_, _ = client.GetEvents(ctx, archive.EventFilterOptionsInput{Address: "B62q..."})
	_, _ = client.GetActions(ctx, archive.ActionFilterOptionsInput{Address: "B62q..."})
	_, _ = client.GetVerificationKeyUpdates(ctx, archive.VerificationKeyUpdateFilterInput{
		VerificationKeyHash: "0x...",
		From:                1,
		To:                  100,
	})
	_, _ = client.GetBlocks(ctx, archive.BlocksOptions{})
	_, _ = client.GetNetworkState(ctx)
}

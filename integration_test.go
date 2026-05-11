package archive

import (
	"context"
	"os"
	"testing"
	"time"
)

const fixtureAddress = "B62qiaEMrWiYdK7LcJ2ScdMyG8LzUxi7yaw17XvBD34on7UKfhAkRML"

// TestIntegration runs only when ARCHIVE_GRAPHQL_URI is set, pointing at a
// live Archive-Node-API server (CI provisions one in .github/workflows/
// integration.yml, backed by the static fixture from o1-labs/Archive-Node-API).
func TestIntegration(t *testing.T) {
	uri := os.Getenv("ARCHIVE_GRAPHQL_URI")
	if uri == "" {
		t.Skip("ARCHIVE_GRAPHQL_URI not set; skipping integration test")
	}

	client := NewClient(
		WithGraphQLURI(uri),
		WithRetries(2),
		WithRetryDelay(time.Second),
	)
	defer client.Close()
	ctx := context.Background()

	t.Run("NetworkState", func(t *testing.T) {
		state, err := client.GetNetworkState(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if state.MaxBlockHeight == nil {
			t.Fatal("maxBlockHeight nil")
		}
		if state.MaxBlockHeight.CanonicalMaxBlockHeight < 0 {
			t.Errorf("canonical = %d", state.MaxBlockHeight.CanonicalMaxBlockHeight)
		}
	})

	t.Run("Events", func(t *testing.T) {
		events, err := client.GetEvents(ctx, EventFilterOptionsInput{
			Address: fixtureAddress,
			Status:  BlockStatusCanonical,
		})
		if err != nil {
			t.Fatal(err)
		}
		if events == nil {
			t.Error("events slice should be non-nil even when empty")
		}
	})

	t.Run("Actions", func(t *testing.T) {
		actions, err := client.GetActions(ctx, ActionFilterOptionsInput{
			Address: fixtureAddress,
			Status:  BlockStatusCanonical,
		})
		if err != nil {
			t.Fatal(err)
		}
		if actions == nil {
			t.Error("actions slice should be non-nil")
		}
	})

	t.Run("Blocks", func(t *testing.T) {
		canonical := true
		limit := 3
		blocks, err := client.GetBlocks(ctx, BlocksOptions{
			Query:  &BlockQueryInput{Canonical: &canonical},
			Limit:  &limit,
			SortBy: BlockSortDesc,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(blocks) >= 2 && blocks[0].BlockHeight < blocks[1].BlockHeight {
			t.Errorf("DESC sort not honored: %d < %d", blocks[0].BlockHeight, blocks[1].BlockHeight)
		}
	})
}

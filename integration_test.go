package archive

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

const fixtureAddress = "B62qiaEMrWiYdK7LcJ2ScdMyG8LzUxi7yaw17XvBD34on7UKfhAkRML"

// fixtureVerificationKeyHash is the single verification key in the upstream
// sample archive dump.
const fixtureVerificationKeyHash = "330109536550383627416201330124291596191867681867265169258470531313815097966"

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
			// The tolerance is deliberately narrow: only the one upstream
			// resolver crash this fixture is known to trigger is excused.
			// Catching every *GraphQLError made this subtest a no-op against
			// any erroring server (#14).
			var gql *GraphQLError
			if errors.As(err, &gql) && strings.Contains(err.Error(), "Cannot read properties of undefined") {
				t.Skipf("known upstream network-service crash against the static fixture: %v", err)
			}
			t.Fatal(err)
		}
		if state.MaxBlockHeight == nil {
			t.Fatal("maxBlockHeight nil")
		}
		// The fixture has 39 blocks, so a real archive reports a real height.
		if state.MaxBlockHeight.CanonicalMaxBlockHeight <= 0 {
			t.Errorf("canonicalMaxBlockHeight = %d, want > 0 against a populated archive",
				state.MaxBlockHeight.CanonicalMaxBlockHeight)
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
		if len(events) == 0 {
			t.Fatal("no event groups for the fixture address; the fixture is known to populate this query")
		}

		// Assert content, not nil-ness. A renamed field or a changed scalar
		// encoding has to fail here.
		var withBlockInfo int
		for _, group := range events {
			if group.BlockInfo == nil {
				continue
			}
			withBlockInfo++
			if group.BlockInfo.Height <= 0 {
				t.Errorf("blockInfo.height = %d, want > 0", group.BlockInfo.Height)
			}
			if group.BlockInfo.StateHash == "" {
				t.Error("blockInfo.stateHash is empty")
			}
			// Unix epoch milliseconds as a decimal string, not RFC3339.
			if _, err := group.BlockInfo.Time(); err != nil {
				t.Errorf("blockInfo.timestamp %q does not parse: %v",
					group.BlockInfo.Timestamp, err)
			}
		}
		if withBlockInfo == 0 {
			t.Error("no event group carried a blockInfo")
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
		if len(actions) == 0 {
			t.Fatal("no action groups for the fixture address; the fixture is known to populate this query")
		}

		var withBlockInfo int
		for _, group := range actions {
			if group.BlockInfo == nil {
				continue
			}
			withBlockInfo++
			if group.BlockInfo.Height <= 0 {
				t.Errorf("blockInfo.height = %d, want > 0", group.BlockInfo.Height)
			}
			if _, err := group.BlockInfo.Time(); err != nil {
				t.Errorf("blockInfo.timestamp %q does not parse: %v",
					group.BlockInfo.Timestamp, err)
			}
		}
		if withBlockInfo == 0 {
			t.Error("no action group carried a blockInfo")
		}
	})

	t.Run("VerificationKeyUpdates", func(t *testing.T) {
		updates, err := client.GetVerificationKeyUpdates(ctx, VerificationKeyUpdateFilterInput{
			VerificationKeyHash: fixtureVerificationKeyHash,
			From:                1,
			To:                  1000,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(updates) == 0 {
			t.Fatal("no verification-key updates; the fixture contains this key, so an empty result is a decode or query regression")
		}
		for _, u := range updates {
			if u.VerificationKeyHash != fixtureVerificationKeyHash {
				t.Errorf("verificationKeyHash = %q, want the requested key", u.VerificationKeyHash)
			}
			if u.BlockInfo.Height <= 0 {
				t.Errorf("blockInfo.height = %d, want > 0", u.BlockInfo.Height)
			}
			if u.Address == "" {
				t.Error("address is empty")
			}
			if _, err := u.BlockInfo.Time(); err != nil {
				t.Errorf("blockInfo.timestamp %q does not parse: %v", u.BlockInfo.Timestamp, err)
			}
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
		if len(blocks) == 0 {
			t.Fatal("no canonical blocks; the fixture holds 24 of them")
		}
		if len(blocks) >= 2 && blocks[0].BlockHeight < blocks[1].BlockHeight {
			t.Errorf("DESC sort not honored: %d < %d", blocks[0].BlockHeight, blocks[1].BlockHeight)
		}

		// The workflow runs the server with ENABLE_BLOCK_TRANSACTION_DETAILS=true,
		// so the decode path for transactions is exercised rather than assumed.
		// Every canonical block in the fixture at height 22 and above carries at
		// least one command, so this cannot be satisfied by an empty result.
		totalTxns := 0
		for _, b := range blocks {
			if b.BlockHeight <= 0 {
				t.Errorf("blockHeight = %d, want > 0", b.BlockHeight)
			}
			if b.StateHash == "" {
				t.Error("stateHash is empty")
			}
			// DateTime is ISO-8601, unlike BlockInfo.Timestamp.
			if _, err := time.Parse(time.RFC3339, b.DateTime); err != nil {
				t.Errorf("dateTime %q is not RFC3339: %v", b.DateTime, err)
			}
			// Coinbase is populated regardless of the transaction-detail flag.
			if _, err := CurrencyFromGraphQL(b.Transactions.Coinbase); err != nil {
				t.Errorf("coinbase %q does not parse as currency: %v",
					b.Transactions.Coinbase, err)
			}
			totalTxns += len(b.Transactions.UserCommands) +
				len(b.Transactions.ZkappCommands) +
				len(b.Transactions.FeeTransfer)
			if b.ParentHash == "" {
				t.Errorf("block %d has an empty ParentHash; is ENABLE_BLOCK_TRANSACTION_DETAILS set?", b.BlockHeight)
			}
		}
		if totalTxns == 0 {
			t.Errorf("no transactions decoded across %d block(s); is ENABLE_BLOCK_TRANSACTION_DETAILS set?", len(blocks))
		}
	})
}

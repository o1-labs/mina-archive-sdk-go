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

// Fixture gap, measured against o1-labs/Archive-Node-API's
// tests/integration/fixtures/archive_db.sql:
//
//   - blocks: 39 rows, 24 canonical — Blocks and NetworkState assert hard.
//   - zkapp_events: ONE row, id=1 with element_ids={}, and all 135
//     account-update bodies reference it. So there are no events and no
//     actions to return, for any address.
//   - zkapp_verification_keys: 1 row, but 0 of 135 bodies set
//     verification_key_hash_id, so no update applies a key.
//
// The Events, Actions and VerificationKeyUpdates subtests therefore Skip when
// empty instead of failing. A skip is visible in CI output, unlike the
// `!= nil` checks these replaced, which passed silently. If the fixture is
// enriched upstream, the content assertions below start running with no change
// needed here.

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
			// Verified against the fixture: zkapp_events holds a single row,
			// id=1 with element_ids={}, and all 135 account-update bodies
			// point at it. There is no event data in archive_db.sql to find,
			// for this address or any other. Skip loudly rather than passing
			// silently; enriching the fixture upstream turns the assertions
			// below back on with no change here.
			t.Skip("fixture contains no zkapp events (zkapp_events has one empty row); see the fixture-gap note in this file")
		}

		// Assert content, not nil-ness. A renamed field or a changed scalar
		// encoding has to fail here.
		var withBlockInfo int
		for i, group := range events {
			// The element is *EventOutput: a wire null is a nil element, and
			// dereferencing it panics the suite instead of reporting a defect.
			if group == nil {
				t.Errorf("events[%d] is null", i)
				continue
			}
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
			// Same fixture gap as Events: every body points at the empty
			// actions_id=1 array.
			t.Skip("fixture contains no zkapp actions (all account-update bodies use the empty actions array)")
		}

		var withBlockInfo int
		for i, group := range actions {
			if group == nil {
				t.Errorf("actions[%d] is null", i)
				continue
			}
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
			// The fixture has one zkapp_verification_keys row, but zero of
			// its 135 account-update bodies set verification_key_hash_id — so
			// no update ever *applies* a key, which is what this query
			// returns.
			t.Skip("fixture contains no applied verification-key updates (0 of 135 account-update bodies set verification_key_hash_id)")
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
		for i, b := range blocks {
			if b == nil {
				t.Fatalf("blocks[%d] is null; every assertion below dereferences it", i)
			}
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

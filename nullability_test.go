package archive

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The positions the schema marks nullable, where a JSON null must not become a
// Go zero value. These tests feed raw JSON rather than building the payload
// from maps, so the wire form is visible and a null is unambiguously a null.

// rawGraphQL answers every request with body verbatim.
func rawGraphQL(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

// Schema 2.0 makes the list elements non-null ([T!]!), so they decode to
// values. Outer nullability stays where the schema keeps it:
// EventOutput.eventData and ActionOutput.actionData may be null as a whole,
// and null must stay distinct from an empty list.
func TestNullEventAndActionDataListStaysDistinctFromEmpty(t *testing.T) {
	t.Run("events", func(t *testing.T) {
		srv := httptest.NewServer(rawGraphQL(
			`{"data":{"events":[{"blockInfo":null,"eventData":null},{"blockInfo":null,"eventData":[]},{"blockInfo":null,"eventData":[{"accountUpdateId":"1","transactionInfo":{"status":"applied","hash":"h","memo":"m","authorizationKind":"Proof","sequenceNumber":0,"zkappAccountUpdateIds":[0,3]},"data":["0x1",""]}]}]}}`,
		))
		defer srv.Close()

		events, err := newTestClient(t, srv).GetEvents(context.Background(), EventFilterOptionsInput{Address: "B62q"})
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(events) != 3 {
			t.Fatalf("want 3 groups, got %d", len(events))
		}
		if events[0].EventData != nil {
			t.Errorf("eventData null decoded to %#v, want nil", events[0].EventData)
		}
		if events[1].EventData == nil || len(events[1].EventData) != 0 {
			t.Errorf("eventData [] decoded to %#v, want a non-nil empty slice", events[1].EventData)
		}
		ed := events[2].EventData[0]
		if len(ed.Data) != 2 || ed.Data[0] != "0x1" || ed.Data[1] != "" {
			t.Errorf("data = %q, want [\"0x1\" \"\"]", ed.Data)
		}
		ids := ed.TransactionInfo.ZkappAccountUpdateIDs
		if len(ids) != 2 || ids[0] != 0 || ids[1] != 3 {
			t.Errorf("zkappAccountUpdateIds = %v, want [0 3]", ids)
		}
	})

	t.Run("actions", func(t *testing.T) {
		srv := httptest.NewServer(rawGraphQL(
			`{"data":{"actions":[{"blockInfo":null,"transactionInfo":null,"actionData":null,"actionState":{}},{"blockInfo":null,"transactionInfo":null,"actionData":[{"accountUpdateId":"1","transactionInfo":null,"data":["0x2"]}],"actionState":{}}]}}`,
		))
		defer srv.Close()

		actions, err := newTestClient(t, srv).GetActions(context.Background(), ActionFilterOptionsInput{Address: "B62q"})
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(actions) != 2 {
			t.Fatalf("want 2 groups, got %d", len(actions))
		}
		if actions[0].ActionData != nil {
			t.Errorf("actionData null decoded to %#v, want nil", actions[0].ActionData)
		}
		if len(actions[1].ActionData) != 1 || len(actions[1].ActionData[0].Data) != 1 || actions[1].ActionData[0].Data[0] != "0x2" {
			t.Errorf("actionData = %#v, want one record with data [\"0x2\"]", actions[1].ActionData)
		}
	})
}

// FailureReason == "" could not distinguish "did not fail" from "failed with
// an empty reason".
func TestNullFailureReasonStaysDistinctFromEmptyString(t *testing.T) {
	srv := httptest.NewServer(rawGraphQL(
		`{"data":{"blocks":[{"blockHeight":1,"creator":"B62q","stateHash":"sh","parentHash":"ph","dateTime":"2023-08-14T22:30:01.000Z","transactions":{"coinbase":"0","userCommands":[{"hash":"h1","kind":"payment","from":"a","to":"b","amount":"1","fee":"1","memo":"","nonce":0,"status":"applied","failureReason":null},{"hash":"h2","kind":"payment","from":"a","to":"b","amount":"1","fee":"1","memo":"","nonce":1,"status":"failed","failureReason":""}],"zkappCommands":[{"hash":"z1","feePayer":"a","fee":"1","memo":"","status":"failed","failureReason":"Cancelled"}],"feeTransfer":[]}}]}}`,
	))
	defer srv.Close()

	blocks, err := newTestClient(t, srv).GetBlocks(context.Background(), BlocksOptions{})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	cmds := blocks[0].Transactions.UserCommands
	if cmds[0].FailureReason != nil {
		t.Errorf("applied command FailureReason = %q, want nil", *cmds[0].FailureReason)
	}
	if cmds[1].FailureReason == nil {
		t.Error("failed-with-empty-reason FailureReason = nil, want a non-nil empty string")
	} else if *cmds[1].FailureReason != "" {
		t.Errorf("FailureReason = %q, want \"\"", *cmds[1].FailureReason)
	}

	zk := blocks[0].Transactions.ZkappCommands[0]
	if zk.FailureReason == nil || *zk.FailureReason != "Cancelled" {
		t.Errorf("zkapp FailureReason = %v, want \"Cancelled\"", zk.FailureReason)
	}
}

// UserCommandOutput.FailureReason (schema 2.1) follows the same convention as
// the block-level UserCommand: nil for "did not fail", a non-nil "" for
// "failed with an empty reason".
func TestNullUserCommandOutputFailureReasonStaysDistinctFromEmptyString(t *testing.T) {
	const blockInfo = `{"height":1,"stateHash":"sh","parentHash":"ph","ledgerHash":"lh","chainStatus":"canonical","timestamp":"0","globalSlotSinceHardfork":0,"globalSlotSinceGenesis":0,"distanceFromMaxBlockHeight":1}`
	srv := httptest.NewServer(rawGraphQL(
		`{"data":{"userCommands":[` +
			`{"blockInfo":` + blockInfo + `,"sequenceNumber":0,"hash":"h1","kind":"payment","feePayer":"a","from":"a","to":"b","amount":"1","fee":"1","memo":"","nonce":0,"status":"applied","failureReason":null},` +
			`{"blockInfo":` + blockInfo + `,"sequenceNumber":1,"hash":"h2","kind":"delegation","feePayer":"a","from":"a","to":"b","amount":"0","fee":"1","memo":"","nonce":1,"status":"failed","failureReason":""}` +
			`]}}`,
	))
	defer srv.Close()

	cmds, err := newTestClient(t, srv).GetUserCommands(context.Background(), UserCommandFilterInput{Address: "a", From: 0, To: 10})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(cmds) != 2 {
		t.Fatalf("want 2 commands, got %d", len(cmds))
	}
	if cmds[0].FailureReason != nil {
		t.Errorf("applied command FailureReason = %q, want nil", *cmds[0].FailureReason)
	}
	if cmds[1].FailureReason == nil {
		t.Error("failed-with-empty-reason FailureReason = nil, want a non-nil empty string")
	} else if *cmds[1].FailureReason != "" {
		t.Errorf("FailureReason = %q, want \"\"", *cmds[1].FailureReason)
	}
}

// The nullable positions round-trip through encoding/json unchanged, so a
// caller re-marshalling a decoded response does not turn nulls into zeroes.
func TestNullablePositionsRoundTrip(t *testing.T) {
	const wire = `{"hash":"z1","feePayer":"a","fee":"1","memo":"","status":"applied","failureReason":null}`

	var zk ZkAppCommand
	if err := json.Unmarshal([]byte(wire), &zk); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := json.Marshal(zk)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != wire {
		t.Errorf("round trip changed the payload:\n got %s\nwant %s", out, wire)
	}
}

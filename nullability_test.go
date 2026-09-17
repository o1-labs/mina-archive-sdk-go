package archive

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The positions the schema marks nullable but the SDK used to model as value
// types, where a JSON null silently became a Go zero value. These tests feed
// raw JSON rather than building the payload from maps, so the wire form is
// visible and a null is unambiguously a null.

// rawGraphQL answers every request with body verbatim.
func rawGraphQL(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

// TestNullDataMemberStaysDistinctFromEmptyString is acceptance criterion 1 of
// issue #9: the wire is ["0x1", null, ""], and null must not arrive as "".
func TestNullDataMemberStaysDistinctFromEmptyString(t *testing.T) {
	srv := httptest.NewServer(rawGraphQL(
		`{"data":{"events":[{"blockInfo":null,"eventData":[{"accountUpdateId":"1","transactionInfo":null,"data":["0x1",null,""]}]}]}}`,
	))
	defer srv.Close()

	events, err := newTestClient(t, srv).GetEvents(context.Background(), EventFilterOptionsInput{Address: "B62q"})
	if err != nil {
		t.Fatalf("a null data member is legal and must decode: %v", err)
	}

	if len(events) != 1 || events[0] == nil {
		t.Fatalf("want one non-nil group, got %#v", events)
	}
	data := events[0].EventData[0].Data
	if len(data) != 3 {
		t.Fatalf("want 3 data members, got %d", len(data))
	}
	if data[0] == nil || *data[0] != "0x1" {
		t.Errorf("Data[0] = %v, want \"0x1\"", data[0])
	}
	if data[1] != nil {
		t.Errorf("Data[1] = %q, want nil — a null member must stay null", *data[1])
	}
	if data[2] == nil {
		t.Error("Data[2] = nil, want a non-nil empty string — \"\" is not null")
	} else if *data[2] != "" {
		t.Errorf("Data[2] = %q, want \"\"", *data[2])
	}
}

// TestNullBlockElementIsNeverAZeroValueBlock is acceptance criterion 2: a null
// [Block] element must not become a block at height 0 with empty hashes, which
// a caller would happily index, count, or feed into a max-height calculation.
func TestNullBlockElementIsNeverAZeroValueBlock(t *testing.T) {
	srv := httptest.NewServer(rawGraphQL(
		`{"data":{"blocks":[null,{"blockHeight":42,"creator":"B62q","stateHash":"sh","parentHash":"ph","dateTime":"2023-08-14T22:30:01.000Z","transactions":{"coinbase":"720000000000","userCommands":[],"zkappCommands":[],"feeTransfer":[]}}]}}`,
	))
	defer srv.Close()

	blocks, err := newTestClient(t, srv).GetBlocks(context.Background(), BlocksOptions{})
	if err != nil {
		t.Fatalf("a null element is legal and must decode: %v", err)
	}

	if len(blocks) != 2 {
		t.Fatalf("want 2 elements, got %d", len(blocks))
	}
	if blocks[0] != nil {
		t.Fatalf("blocks[0] = %+v, want nil — a null block must not become height %d",
			*blocks[0], blocks[0].BlockHeight)
	}
	if blocks[1] == nil || blocks[1].BlockHeight != 42 {
		t.Errorf("blocks[1] = %v, want the real block at height 42", blocks[1])
	}
}

func TestNullEventAndActionElementsDecodeToNil(t *testing.T) {
	t.Run("events", func(t *testing.T) {
		srv := httptest.NewServer(rawGraphQL(`{"data":{"events":[null]}}`))
		defer srv.Close()

		events, err := newTestClient(t, srv).GetEvents(context.Background(), EventFilterOptionsInput{Address: "B62q"})
		if err != nil {
			t.Fatalf("a null element is legal and must decode: %v", err)
		}
		if len(events) != 1 {
			t.Fatalf("want the null kept, not elided: got %d elements", len(events))
		}
		if events[0] != nil {
			t.Errorf("events[0] = %+v, want nil", *events[0])
		}
	})

	t.Run("actions", func(t *testing.T) {
		srv := httptest.NewServer(rawGraphQL(`{"data":{"actions":[null]}}`))
		defer srv.Close()

		actions, err := newTestClient(t, srv).GetActions(context.Background(), ActionFilterOptionsInput{Address: "B62q"})
		if err != nil {
			t.Fatalf("a null element is legal and must decode: %v", err)
		}
		if len(actions) != 1 {
			t.Fatalf("want the null kept, not elided: got %d elements", len(actions))
		}
		if actions[0] != nil {
			t.Errorf("actions[0] = %+v, want nil", *actions[0])
		}
	})
}

// A null zkappAccountUpdateIds member used to decode to 0, indistinguishable
// from a real account-update id of 0.
func TestNullZkappAccountUpdateIDStaysDistinctFromZero(t *testing.T) {
	srv := httptest.NewServer(rawGraphQL(
		`{"data":{"events":[{"blockInfo":null,"eventData":[{"accountUpdateId":"1","transactionInfo":{"status":"applied","hash":"h","memo":"m","authorizationKind":"Proof","sequenceNumber":0,"zkappAccountUpdateIds":[0,null,3]},"data":["0x1"]}]}]}}`,
	))
	defer srv.Close()

	events, err := newTestClient(t, srv).GetEvents(context.Background(), EventFilterOptionsInput{Address: "B62q"})
	if err != nil {
		t.Fatalf("a null id member is legal and must decode: %v", err)
	}

	ids := events[0].EventData[0].TransactionInfo.ZkappAccountUpdateIDs
	if len(ids) != 3 {
		t.Fatalf("want 3 ids, got %d", len(ids))
	}
	if ids[0] == nil || *ids[0] != 0 {
		t.Errorf("ids[0] = %v, want a real id 0", ids[0])
	}
	if ids[1] != nil {
		t.Errorf("ids[1] = %d, want nil — a null id must not become 0", *ids[1])
	}
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

// The nullable positions round-trip through encoding/json unchanged, so a
// caller re-marshalling a decoded response does not turn nulls into zeroes.
func TestNullablePositionsRoundTrip(t *testing.T) {
	const wire = `{"accountUpdateId":"1","transactionInfo":null,"data":["0x1",null,""]}`

	var ed EventData
	if err := json.Unmarshal([]byte(wire), &ed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := json.Marshal(ed)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != wire {
		t.Errorf("round trip changed the payload:\n got %s\nwant %s", out, wire)
	}
}

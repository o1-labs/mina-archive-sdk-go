package archive

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newTestClient wires a Client to the given httptest.Server with no retry
// backoff so tests stay fast.
func newTestClient(t *testing.T, srv *httptest.Server, opts ...ClientOption) *Client {
	t.Helper()
	base := []ClientOption{
		WithGraphQLURI(srv.URL),
		WithRetryDelay(0),
	}
	return NewClient(append(base, opts...)...)
}

// graphqlOK wraps a `data` payload in the GraphQL response envelope.
func graphqlOK(data any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}
}

func TestGetEventsHappyPath(t *testing.T) {
	srv := httptest.NewServer(graphqlOK(map[string]any{
		"events": []map[string]any{{
			"blockInfo": map[string]any{
				"height":                     100,
				"stateHash":                  "sh",
				"parentHash":                 "ph",
				"ledgerHash":                 "lh",
				"chainStatus":                "canonical",
				"timestamp":                  "0",
				"globalSlotSinceHardfork":    0,
				"globalSlotSinceGenesis":     0,
				"distanceFromMaxBlockHeight": 1,
			},
			"eventData": []map[string]any{
				{"accountUpdateId": "1", "transactionInfo": nil, "data": []string{"0x1"}},
			},
		}},
	}))
	defer srv.Close()
	client := newTestClient(t, srv)

	events, err := client.GetEvents(context.Background(), EventFilterOptionsInput{Address: "B62q"})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event group, got %d", len(events))
	}
	if events[0].BlockInfo.Height != 100 {
		t.Errorf("height = %d", events[0].BlockInfo.Height)
	}
}

func TestGetNetworkState(t *testing.T) {
	srv := httptest.NewServer(graphqlOK(map[string]any{
		"networkState": map[string]any{
			"maxBlockHeight": map[string]any{
				"canonicalMaxBlockHeight": 1000,
				"pendingMaxBlockHeight":   1010,
			},
		},
	}))
	defer srv.Close()
	client := newTestClient(t, srv)

	state, err := client.GetNetworkState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.MaxBlockHeight.CanonicalMaxBlockHeight != 1000 {
		t.Errorf("canonical = %d", state.MaxBlockHeight.CanonicalMaxBlockHeight)
	}
}

func TestGetActionsHappyPath(t *testing.T) {
	srv := httptest.NewServer(graphqlOK(map[string]any{
		"actions": []map[string]any{{
			"blockInfo":       nil,
			"transactionInfo": nil,
			"actionData":      []any{},
			"actionState": map[string]any{
				"actionStateOne":   "a",
				"actionStateTwo":   nil,
				"actionStateThree": nil,
				"actionStateFour":  nil,
				"actionStateFive":  nil,
			},
		}},
	}))
	defer srv.Close()
	client := newTestClient(t, srv)

	actions, err := client.GetActions(context.Background(), ActionFilterOptionsInput{Address: "B62q"})
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 group, got %d", len(actions))
	}
	if actions[0].ActionState.ActionStateOne == nil || *actions[0].ActionState.ActionStateOne != "a" {
		t.Error("actionStateOne not decoded")
	}
}

func TestGetBlocksPassesOptionalFiltersAsNull(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body graphqlRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		captured = body.Variables
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"blocks": []any{}}})
	}))
	defer srv.Close()
	client := newTestClient(t, srv)

	if _, err := client.GetBlocks(context.Background(), BlocksOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"query", "limit", "sortBy"} {
		if v, ok := captured[k]; !ok {
			t.Errorf("variable %q missing", k)
		} else if v != nil {
			t.Errorf("variable %q = %v, want nil", k, v)
		}
	}
}

func TestGraphQLErrorIsNotRetried(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]string{{"message": "bad input"}},
		})
	}))
	defer srv.Close()
	client := newTestClient(t, srv, WithRetries(3))

	_, err := client.GetEvents(context.Background(), EventFilterOptionsInput{Address: ""})
	var gqlErr *GraphQLError
	if !errors.As(err, &gqlErr) {
		t.Fatalf("expected *GraphQLError, got %v", err)
	}
	if calls != 1 {
		t.Errorf("GraphQL errors must not retry; got %d calls", calls)
	}
}

func TestTransientHTTP500ThenSuccess(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls < 2 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"networkState": map[string]any{
					"maxBlockHeight": map[string]any{
						"canonicalMaxBlockHeight": 1, "pendingMaxBlockHeight": 2,
					},
				},
			},
		})
	}))
	defer srv.Close()
	client := newTestClient(t, srv, WithRetries(3))

	state, err := client.GetNetworkState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.MaxBlockHeight.CanonicalMaxBlockHeight != 1 {
		t.Errorf("canonical = %d", state.MaxBlockHeight.CanonicalMaxBlockHeight)
	}
}

func TestPersistentHTTPFailureGivesConnectionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "fail", http.StatusBadGateway)
	}))
	defer srv.Close()
	client := newTestClient(t, srv, WithRetries(2))

	_, err := client.GetNetworkState(context.Background())
	var connErr *ConnectionError
	if !errors.As(err, &connErr) {
		t.Fatalf("expected *ConnectionError, got %v", err)
	}
	if connErr.Retries != 2 {
		t.Errorf("retries = %d", connErr.Retries)
	}
}

func TestMissingDataField(t *testing.T) {
	srv := httptest.NewServer(graphqlOK(map[string]any{}))
	defer srv.Close()
	client := newTestClient(t, srv)

	_, err := client.GetEvents(context.Background(), EventFilterOptionsInput{Address: "B62q"})
	var miss *MissingFieldError
	if !errors.As(err, &miss) {
		t.Errorf("expected *MissingFieldError, got %v", err)
	}
}

func TestCustomHeaderForwarded(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-Api-Key")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"networkState": map[string]any{"maxBlockHeight": map[string]any{"canonicalMaxBlockHeight": 1, "pendingMaxBlockHeight": 1}},
		}})
	}))
	defer srv.Close()
	client := newTestClient(t, srv, WithHeader("X-Api-Key", "secret"))

	if _, err := client.GetNetworkState(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got != "secret" {
		t.Errorf("header = %q", got)
	}
}

func TestContextCancellationDuringBackoff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	client := newTestClient(t, srv, WithRetries(5), WithRetryDelay(100*time.Millisecond))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := client.GetNetworkState(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

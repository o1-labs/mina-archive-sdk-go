package archive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestGetVerificationKeyUpdatesHappyPath(t *testing.T) {
	srv := httptest.NewServer(graphqlOK(map[string]any{
		"verificationKeyUpdates": []any{
			map[string]any{
				"accountUpdateId":     "42",
				"address":             "B62qtest",
				"tokenId":             "wSHV2S4qX9jFsLjQo8r1BsMLH2ZRKsZx6EJd1sbozGPieEC4Jf",
				"verificationKeyHash": "3301095365503836274162013301242915961918676818672",
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
				"transactionInfo": map[string]any{
					"status":                "applied",
					"hash":                  "txhash",
					"memo":                  "",
					"authorizationKind":     "Proof",
					"sequenceNumber":        0,
					"zkappAccountUpdateIds": []any{42},
				},
			},
		},
	}))
	defer srv.Close()
	client := newTestClient(t, srv)

	updates, err := client.GetVerificationKeyUpdates(context.Background(), VerificationKeyUpdateFilterInput{
		VerificationKeyHash: "3301095365503836274162013301242915961918676818672",
		From:                1,
		To:                  1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 1 {
		t.Fatalf("len(updates) = %d, want 1", len(updates))
	}
	if updates[0].Address != "B62qtest" {
		t.Errorf("address = %q", updates[0].Address)
	}
	if updates[0].BlockInfo.Height != 100 {
		t.Errorf("height = %d", updates[0].BlockInfo.Height)
	}
}

func TestVerificationKeyFilterSendsRequiredRange(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body graphqlRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		captured = body.Variables
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"verificationKeyUpdates": []any{}},
		})
	}))
	defer srv.Close()
	client := newTestClient(t, srv)

	if _, err := client.GetVerificationKeyUpdates(context.Background(), VerificationKeyUpdateFilterInput{
		VerificationKeyHash: "vk",
		From:                10,
		To:                  20,
	}); err != nil {
		t.Fatal(err)
	}
	input, ok := captured["input"].(map[string]any)
	if !ok {
		t.Fatalf("input variable missing or not an object: %v", captured)
	}
	if input["verificationKeyHash"] != "vk" {
		t.Errorf("verificationKeyHash = %v", input["verificationKeyHash"])
	}
	// The range is required, so it is sent even at the zero value of Status.
	if input["from"] != float64(10) || input["to"] != float64(20) {
		t.Errorf("range = %v..%v, want 10..20", input["from"], input["to"])
	}
	if _, present := input["status"]; present {
		t.Error("status should be omitted when unset")
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

// The endpoint is the server's root path "/" (#6). Pointing the client at
// "/graphql" gets a 404 whose body is HTML, and that used to be reported as a
// JSON "decode response" failure, which sent people looking in the wrong
// place. The status is now read before the body is parsed.
func TestHTTPErrorOnNonJSON404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("<!DOCTYPE html><html><body>Not Found</body></html>"))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	defer client.Close()

	_, err := client.GetNetworkState(context.Background())
	if err == nil {
		t.Fatal("expected an error from a 404")
	}

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected *HTTPError, got %T: %v", err, err)
	}
	if httpErr.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want 404", httpErr.StatusCode)
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error message %q should mention 404", err.Error())
	}
	if strings.Contains(err.Error(), "decode response") {
		t.Errorf("a 404 should not be reported as a decode failure: %q", err.Error())
	}
}

// The same 404 with a JSON body must also be an *HTTPError, so callers have
// one type to match on regardless of what the failing route happens to serve.
func TestHTTPErrorOnJSON404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	defer client.Close()

	_, err := client.GetNetworkState(context.Background())
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected *HTTPError, got %T: %v", err, err)
	}
	if httpErr.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want 404", httpErr.StatusCode)
	}
}

// WithTimeout used to do c.httpClient.Timeout = d, writing into whatever the
// client currently pointed at — including an *http.Client the caller supplied
// and may share across a connection pool (#11).
func TestWithTimeoutDoesNotMutateCallerClient(t *testing.T) {
	hc := &http.Client{}
	client := NewClient(WithHTTPClient(hc), WithTimeout(5*time.Second))
	defer client.Close()

	if hc.Timeout != 0 {
		t.Errorf("caller's http.Client.Timeout = %s, want 0 — the SDK must not write to it", hc.Timeout)
	}
}

// Option order must not change the outcome. Before, one order silently
// discarded WithTimeout and the other wrote through into the caller's object.
func TestTimeoutOptionOrderIsIrrelevant(t *testing.T) {
	hcA := &http.Client{}
	hcB := &http.Client{}

	a := NewClient(WithTimeout(5*time.Second), WithHTTPClient(hcA))
	defer a.Close()
	b := NewClient(WithHTTPClient(hcB), WithTimeout(5*time.Second))
	defer b.Close()

	if hcA.Timeout != 0 || hcB.Timeout != 0 {
		t.Errorf("caller clients mutated: A=%s B=%s, want 0/0", hcA.Timeout, hcB.Timeout)
	}
	if a.timeout != b.timeout {
		t.Errorf("option order changed the effective timeout: %s vs %s", a.timeout, b.timeout)
	}
	if a.timeout != 5*time.Second {
		t.Errorf("effective timeout = %s, want 5s", a.timeout)
	}
}

// The timeout must still actually apply — moving it off the http.Client must
// not quietly disable it.
func TestTimeoutStillAppliesPerRequest(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	hc := &http.Client{}
	client := NewClient(
		WithGraphQLURI(srv.URL),
		WithHTTPClient(hc),
		WithTimeout(100*time.Millisecond),
		WithRetries(1),
	)
	defer client.Close()

	start := time.Now()
	_, err := client.GetNetworkState(context.Background())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected the request to time out")
	}
	if elapsed > 3*time.Second {
		t.Errorf("took %s — the per-request deadline does not appear to apply", elapsed)
	}
	if hc.Timeout != 0 {
		t.Errorf("caller's client was mutated during the request: %s", hc.Timeout)
	}
}

// Close must not reach into a pool the caller still owns.
func TestCloseDoesNotTouchCallerPool(t *testing.T) {
	hc := &http.Client{Transport: &countingTransport{}}
	client := NewClient(WithHTTPClient(hc))
	client.Close()

	tr, ok := hc.Transport.(*countingTransport)
	if !ok {
		t.Fatal("unexpected transport")
	}
	if tr.closeIdleCalls != 0 {
		t.Errorf("CloseIdleConnections called %d time(s) on a borrowed pool, want 0", tr.closeIdleCalls)
	}

	// The default client is ours, so closing it is still allowed.
	own := NewClient()
	own.Close()
}

type countingTransport struct {
	closeIdleCalls int
}

func (t *countingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("not used")
}

func (t *countingTransport) CloseIdleConnections() { t.closeIdleCalls++ }

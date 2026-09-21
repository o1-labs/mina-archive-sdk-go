package archive

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Every error path a query can take must land on an exported type. The README
// presents its taxonomy as exhaustive; before this, four paths in ExecuteQuery
// and one per typed method returned bare fmt.Errorf values, so "exhaustive"
// was a claim the code did not keep.

func TestErrorTaxonomyIsExhaustive(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		wantAs  func(error) bool
		wantMsg string
	}{
		{
			name:   "404 with an HTML body",
			status: http.StatusNotFound,
			body:   "<html><body>Cannot POST /</body></html>",
			wantAs: func(err error) bool {
				var e *HTTPError
				return errors.As(err, &e) && e.StatusCode == 404
			},
		},
		{
			name:   "404 with a JSON body",
			status: http.StatusNotFound,
			body:   `{"message":"Not Found"}`,
			wantAs: func(err error) bool {
				var e *HTTPError
				return errors.As(err, &e)
			},
		},
		{
			name:   "200 with a body that is not a GraphQL response",
			status: http.StatusOK,
			body:   "not json at all",
			wantAs: func(err error) bool {
				var e *DecodeError
				return errors.As(err, &e)
			},
		},
		{
			// Valid JSON with neither data nor errors. This used to report
			// "unexpected end of JSON input" — a parse complaint about a
			// payload that parses fine.
			name:   "200 with a bare {} body",
			status: http.StatusOK,
			body:   `{}`,
			wantAs: func(err error) bool {
				var e *MissingFieldError
				return errors.As(err, &e) && e.Field == "data"
			},
		},
		{
			name:   "200 whose data does not fit the typed result",
			status: http.StatusOK,
			body:   `{"data":{"events":"a string, not a list"}}`,
			wantAs: func(err error) bool {
				var e *DecodeError
				return errors.As(err, &e)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			_, err := newTestClient(t, srv).GetEvents(context.Background(), EventFilterOptionsInput{Address: "B62q"})
			if err == nil {
				t.Fatal("want an error")
			}
			if !tc.wantAs(err) {
				t.Errorf("error %T does not match the expected exported type: %v", err, err)
			}
		})
	}
}

// Variables that cannot be marshalled are the fourth case the issue names.
// ExecuteQuery is the only way to supply them, since the typed methods build
// their own.
func TestUnmarshalableVariablesAreInvalidInput(t *testing.T) {
	srv := httptest.NewServer(graphqlOK(map[string]any{}))
	defer srv.Close()

	// A channel has no JSON representation.
	_, err := newTestClient(t, srv).ExecuteQuery(
		context.Background(), "query { networkState { maxBlockHeight { canonicalMaxBlockHeight } } }",
		map[string]any{"bad": make(chan int)}, "ExecuteQuery",
	)

	var invErr *InvalidInputError
	if !errors.As(err, &invErr) {
		t.Fatalf("error %T, want *InvalidInputError: %v", err, err)
	}
	if invErr.Field != "variables" {
		t.Errorf("Field = %q, want \"variables\"", invErr.Field)
	}
}

// GraphQL Int is signed 32-bit. An out-of-range bound must be rejected before
// any HTTP request happens, not after a round trip to the server's validator.
func TestIntBoundsRejectedWithoutARequest(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"events":[]}}`))
	}))
	defer srv.Close()
	client := newTestClient(t, srv)

	tooBig := 1 << 31
	_, err := client.GetEvents(context.Background(), EventFilterOptionsInput{
		Address: "B62q",
		To:      IntPtr(tooBig),
	})

	var invErr *InvalidInputError
	if !errors.As(err, &invErr) {
		t.Fatalf("error %T, want *InvalidInputError: %v", err, err)
	}
	if invErr.Field != "to" {
		t.Errorf("Field = %q, want \"to\"", invErr.Field)
	}
	if called {
		t.Error("the client sent an HTTP request for input it could reject locally")
	}

	// The same bound on every other Int-typed input.
	if _, err := client.GetActions(context.Background(), ActionFilterOptionsInput{
		Address: "B62q", From: IntPtr(-tooBig - 1),
	}); !errors.As(err, &invErr) {
		t.Errorf("GetActions from: %T", err)
	}
	if _, err := client.GetBlocks(context.Background(), BlocksOptions{
		Limit: IntPtr(tooBig),
	}); !errors.As(err, &invErr) {
		t.Errorf("GetBlocks limit: %T", err)
	}
	if _, err := client.GetBlocks(context.Background(), BlocksOptions{
		Query: &BlockQueryInput{BlockHeightGte: IntPtr(tooBig)},
	}); !errors.As(err, &invErr) {
		t.Errorf("GetBlocks blockHeight_gte: %T", err)
	}
	if _, err := client.GetVerificationKeyUpdates(context.Background(), VerificationKeyUpdateFilterInput{
		VerificationKeyHash: "0x", From: 1, To: tooBig,
	}); !errors.As(err, &invErr) {
		t.Errorf("GetVerificationKeyUpdates to: %T", err)
	}
	if called {
		t.Error("the client sent an HTTP request for input it could reject locally")
	}

	// A value inside the range still goes out.
	if _, err := client.GetEvents(context.Background(), EventFilterOptionsInput{
		Address: "B62q", To: IntPtr(1<<31 - 1),
	}); err != nil {
		t.Errorf("a bound at MaxInt32 must be accepted: %v", err)
	}
	if !called {
		t.Error("the in-range request never reached the server")
	}
}

// A cancellation during backoff used to surface as a raw ctx.Err() on some
// attempts and a *ConnectionError on the last one, so the type a caller saw
// depended on timing.
func TestCancellationDuringBackoffIsAlwaysConnectionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	client := NewClient(
		WithGraphQLURI(srv.URL),
		WithRetries(5),
		WithRetryDelay(time.Second),
	)

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	_, err := client.GetEvents(ctx, EventFilterOptionsInput{Address: "B62q"})

	var connErr *ConnectionError
	if !errors.As(err, &connErr) {
		t.Fatalf("error %T, want *ConnectionError: %v", err, err)
	}
	if connErr.QueryName != "GetEvents" {
		t.Errorf("QueryName = %q, want \"GetEvents\" — the query name must survive", connErr.QueryName)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false; Unwrap must expose the cause")
	}
}

// WithTimeout bounds one HTTP request; it never covered the sleep between
// attempts, so a server answering Retry-After: 3600 parked the call for an
// hour whatever the timeout said. Above the ceiling the call returns at once.
func TestRetryAfterBeyondTheCeilingReturnsAtOnce(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"errors":[{"message":"Too many requests"}]}`))
	}))
	defer srv.Close()

	c := NewClient(
		WithGraphQLURI(srv.URL),
		WithRetries(3),
		WithRetryDelay(0),
		WithMaxRetryAfter(60*time.Second),
	)

	started := time.Now()
	_, err := c.GetNetworkState(context.Background())
	elapsed := time.Since(started)

	var rateErr *RateLimitError
	if !errors.As(err, &rateErr) {
		t.Fatalf("error = %v, want *RateLimitError", err)
	}
	if rateErr.RetryAfter != 3600*time.Second {
		t.Errorf("RetryAfter = %v, want the server's 3600s", rateErr.RetryAfter)
	}
	// The point of the fix: it returned instead of sleeping.
	if elapsed > 5*time.Second {
		t.Errorf("call took %v; it slept on Retry-After instead of returning", elapsed)
	}
}

// Within the ceiling, Retry-After is still honoured and the retry still wins.
func TestRetryAfterWithinTheCeilingIsHonoured(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"errors":[{"message":"Too many requests"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"networkState":{"maxBlockHeight":{"canonicalMaxBlockHeight":42,"pendingMaxBlockHeight":43}}}}`))
	}))
	defer srv.Close()

	c := NewClient(
		WithGraphQLURI(srv.URL),
		WithRetries(3),
		WithRetryDelay(0),
		WithMaxRetryAfter(60*time.Second),
	)
	state, err := c.GetNetworkState(context.Background())
	if err != nil {
		t.Fatalf("GetNetworkState: %v", err)
	}
	if state.MaxBlockHeight == nil || state.MaxBlockHeight.CanonicalMaxBlockHeight != 42 {
		t.Errorf("canonical max height = %+v, want 42", state.MaxBlockHeight)
	}
	if calls != 2 {
		t.Errorf("server saw %d calls, want 2 (one 429, one success)", calls)
	}
}

package archive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// DefaultGraphQLURI points at the local development Archive-Node-API server.
const DefaultGraphQLURI = "http://localhost:8080/"

// ClientOption configures a Client at construction time.
type ClientOption func(*Client)

// WithGraphQLURI sets the GraphQL endpoint URI.
func WithGraphQLURI(uri string) ClientOption {
	return func(c *Client) { c.uri = uri }
}

// WithRetries sets the total number of attempts (including the initial try).
// Must be at least 1.
func WithRetries(n int) ClientOption {
	return func(c *Client) { c.retries = n }
}

// WithRetryDelay sets the delay between retries.
func WithRetryDelay(d time.Duration) ClientOption {
	return func(c *Client) { c.retryDelay = d }
}

// WithTimeout sets the per-request HTTP timeout.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) { c.httpClient.Timeout = d }
}

// WithHTTPClient supplies a custom *http.Client (replaces the default).
// Useful for tests and for sharing connection pools.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

// WithHeader attaches a header to every outgoing request. Repeat to add
// multiple headers.
func WithHeader(key, value string) ClientOption {
	return func(c *Client) {
		if c.headers == nil {
			c.headers = http.Header{}
		}
		c.headers.Set(key, value)
	}
}

// WithLogger redirects the client's connection-attempt logs.
func WithLogger(l *log.Logger) ClientOption {
	return func(c *Client) { c.logger = l }
}

// Client is a Mina Archive Node GraphQL client.
type Client struct {
	uri        string
	retries    int
	retryDelay time.Duration
	headers    http.Header
	httpClient *http.Client
	logger     *log.Logger
}

// NewClient creates a Client with the given options. Sensible defaults: the
// local URI, 3 retries with 5s backoff, 30s per-request timeout.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		uri:        DefaultGraphQLURI,
		retries:    3,
		retryDelay: 5 * time.Second,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.retries < 1 {
		c.retries = 1
	}
	return c
}

// Close releases idle connections held by the client's HTTP transport.
func (c *Client) Close() {
	c.httpClient.CloseIdleConnections()
}

// GraphQLURI returns the configured endpoint URI.
func (c *Client) GraphQLURI() string { return c.uri }

// ExecuteQuery runs an arbitrary GraphQL query through the client's retry
// path and returns the raw `data` field as json.RawMessage. Prefer the typed
// methods below for known queries.
//
// queryName is used only in error messages and debug logs.
func (c *Client) ExecuteQuery(ctx context.Context, query string, variables map[string]any, queryName string) (json.RawMessage, error) {
	payload, err := json.Marshal(graphqlRequest{Query: query, Variables: variables})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= c.retries; attempt++ {
		c.logf("GraphQL %s attempt %d/%d", queryName, attempt, c.retries)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.uri, bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		for k, vs := range c.headers {
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			c.logf("GraphQL %s transport error: %v", queryName, err)
			if attempt < c.retries {
				if waitErr := sleepCtx(ctx, c.retryDelay); waitErr != nil {
					return nil, waitErr
				}
			}
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if attempt < c.retries {
				if waitErr := sleepCtx(ctx, c.retryDelay); waitErr != nil {
					return nil, waitErr
				}
			}
			continue
		}

		if resp.StatusCode >= 500 || resp.StatusCode == http.StatusRequestTimeout {
			// Retryable HTTP failures.
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
			c.logf("GraphQL %s HTTP %d", queryName, resp.StatusCode)
			if attempt < c.retries {
				if waitErr := sleepCtx(ctx, c.retryDelay); waitErr != nil {
					return nil, waitErr
				}
			}
			continue
		}

		var gql graphqlResponse
		if err := json.Unmarshal(body, &gql); err != nil {
			return nil, fmt.Errorf("decode response: %w (body: %s)", err, truncate(string(body), 200))
		}

		// GraphQL-level errors are not retried — they are deterministic.
		if len(gql.Errors) > 0 {
			return nil, &GraphQLError{QueryName: queryName, Errors: gql.Errors}
		}

		if resp.StatusCode >= 400 {
			// 4xx with no GraphQL error array — surface as a hard failure.
			return nil, fmt.Errorf("HTTP %d in %s: %s", resp.StatusCode, queryName, truncate(string(body), 200))
		}

		return gql.Data, nil
	}

	return nil, &ConnectionError{QueryName: queryName, Retries: c.retries, LastError: lastErr}
}

// -- Typed queries --

// GetEvents queries archived events for a zkApp account.
func (c *Client) GetEvents(ctx context.Context, in EventFilterOptionsInput) ([]EventOutput, error) {
	data, err := c.ExecuteQuery(ctx, queryEvents, map[string]any{"input": in.toMap()}, "GetEvents")
	if err != nil {
		return nil, err
	}
	var result struct {
		Events *[]EventOutput `json:"events"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode GetEvents: %w", err)
	}
	if result.Events == nil {
		return nil, &MissingFieldError{QueryName: "GetEvents", Field: "events"}
	}
	return *result.Events, nil
}

// GetActions queries archived actions for a zkApp account.
func (c *Client) GetActions(ctx context.Context, in ActionFilterOptionsInput) ([]ActionOutput, error) {
	data, err := c.ExecuteQuery(ctx, queryActions, map[string]any{"input": in.toMap()}, "GetActions")
	if err != nil {
		return nil, err
	}
	var result struct {
		Actions *[]ActionOutput `json:"actions"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode GetActions: %w", err)
	}
	if result.Actions == nil {
		return nil, &MissingFieldError{QueryName: "GetActions", Field: "actions"}
	}
	return *result.Actions, nil
}

// GetNetworkState returns the archive's current high-water mark.
func (c *Client) GetNetworkState(ctx context.Context) (*NetworkStateOutput, error) {
	data, err := c.ExecuteQuery(ctx, queryNetworkState, nil, "GetNetworkState")
	if err != nil {
		return nil, err
	}
	var result struct {
		NetworkState *NetworkStateOutput `json:"networkState"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode GetNetworkState: %w", err)
	}
	if result.NetworkState == nil {
		return nil, &MissingFieldError{QueryName: "GetNetworkState", Field: "networkState"}
	}
	return result.NetworkState, nil
}

// BlocksOptions are the optional filters for GetBlocks.
type BlocksOptions struct {
	Query  *BlockQueryInput
	Limit  *int
	SortBy BlockSortBy
}

// GetBlocks queries blocks by the given filters. Pass an empty BlocksOptions{}
// to get the default server-side selection.
func (c *Client) GetBlocks(ctx context.Context, opts BlocksOptions) ([]Block, error) {
	vars := map[string]any{
		"query":  nil,
		"limit":  nil,
		"sortBy": nil,
	}
	if opts.Query != nil {
		vars["query"] = opts.Query.toMap()
	}
	if opts.Limit != nil {
		vars["limit"] = *opts.Limit
	}
	if opts.SortBy != "" {
		vars["sortBy"] = string(opts.SortBy)
	}

	data, err := c.ExecuteQuery(ctx, queryBlocks, vars, "GetBlocks")
	if err != nil {
		return nil, err
	}
	var result struct {
		Blocks *[]Block `json:"blocks"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode GetBlocks: %w", err)
	}
	if result.Blocks == nil {
		return nil, &MissingFieldError{QueryName: "GetBlocks", Field: "blocks"}
	}
	return *result.Blocks, nil
}

// GetVerificationKeyUpdates finds applied account updates that set the given
// verification key. The block range is required and the server caps its width,
// so walk a wide history in pages rather than in one call.
func (c *Client) GetVerificationKeyUpdates(ctx context.Context, in VerificationKeyUpdateFilterInput) ([]VerificationKeyUpdate, error) {
	data, err := c.ExecuteQuery(ctx, queryVerificationKeyUpdates, map[string]any{"input": in.toMap()}, "GetVerificationKeyUpdates")
	if err != nil {
		return nil, err
	}
	var result struct {
		VerificationKeyUpdates *[]VerificationKeyUpdate `json:"verificationKeyUpdates"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode GetVerificationKeyUpdates: %w", err)
	}
	if result.VerificationKeyUpdates == nil {
		return nil, &MissingFieldError{QueryName: "GetVerificationKeyUpdates", Field: "verificationKeyUpdates"}
	}
	return *result.VerificationKeyUpdates, nil
}

// IntPtr / BoolPtr are convenience helpers for the optional pointer fields
// on filter structs. They avoid the `x := 5; &x` ceremony in call sites.
func IntPtr(v int) *int    { return &v }
func BoolPtr(v bool) *bool { return &v }

func (c *Client) logf(format string, args ...any) {
	if c.logger == nil {
		return
	}
	c.logger.Printf(format, args...)
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

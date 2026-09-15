package archive

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// GraphQLError is returned when the server replies with a non-empty `errors`
// array. GraphQL-level errors are not retried by the client.
//
// Data carries the response's `data` field when the server sent one alongside
// the errors. HTTP 200 with both is a normal GraphQL outcome: the root lists
// and most of their fields are nullable, so a field-level resolver error
// nullifies a sub-tree rather than the whole response. Unmarshal it to recover
// the rows that did arrive. It is nil when the server sent `"data": null`,
// which is a total failure rather than a partial one.
type GraphQLError struct {
	QueryName string
	Errors    []GraphQLErrorEntry
	Data      json.RawMessage
}

// HasPartialData reports whether the server returned usable data alongside the
// errors.
func (e *GraphQLError) HasPartialData() bool {
	return len(e.Data) > 0 && string(e.Data) != "null"
}

// Contract error codes published by Archive-Node-API in extensions.code.
//
// These are the supported way to branch on a failure. Message text is
// deliberately minimal — the server blocks GraphQL field suggestions — and
// carries no stability promise, so do not match on it.
const (
	// CodeBlockRangeError means the requested range exceeds BLOCK_RANGE_SIZE.
	// Narrow the range; retrying unchanged will fail again.
	CodeBlockRangeError = "BLOCK_RANGE_ERROR"
	// CodeActionStateNotFound means the action state is not in the archive.
	CodeActionStateNotFound = "ACTION_STATE_NOT_FOUND"
	// CodeActionStateOutOfRange means the action state falls outside the
	// requested block range.
	CodeActionStateOutOfRange = "ACTION_STATE_OUT_OF_RANGE"
	// CodeRateLimited means the request was rate limited. See RateLimitError.
	CodeRateLimited = "RATE_LIMITED"
)

// GraphQLErrorLocation is one entry of an error's `locations` array.
type GraphQLErrorLocation struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// GraphQLErrorEntry is one item from the GraphQL response's `errors` field.
//
// Extensions is kept whole rather than flattened, so a caller can read fields
// the SDK does not model yet. Note the API answers every GraphQL-level error
// with HTTP 200 — extensions["status"] is a payload field, not the HTTP
// status — so this struct is the only place the information exists.
type GraphQLErrorEntry struct {
	Message    string                 `json:"message"`
	Extensions map[string]any         `json:"extensions,omitempty"`
	Path       []any                  `json:"path,omitempty"`
	Locations  []GraphQLErrorLocation `json:"locations,omitempty"`
}

// Code returns extensions.code, or "" when the server sent none.
//
// A masked error has no extensions at all — the server runs with
// maskedErrors and isDev false — so "" means "no code was sent", never "no
// error occurred".
func (e GraphQLErrorEntry) Code() string {
	if e.Extensions == nil {
		return ""
	}
	code, _ := e.Extensions["code"].(string)
	return code
}

// Code returns the first non-empty extensions.code among the entries, or "".
func (e *GraphQLError) Code() string {
	for _, entry := range e.Errors {
		if code := entry.Code(); code != "" {
			return code
		}
	}
	return ""
}

// Codes returns every non-empty extensions.code among the entries, in order.
func (e *GraphQLError) Codes() []string {
	var codes []string
	for _, entry := range e.Errors {
		if code := entry.Code(); code != "" {
			codes = append(codes, code)
		}
	}
	return codes
}

// HasCode reports whether any entry carries the given extensions.code.
func (e *GraphQLError) HasCode(code string) bool {
	for _, entry := range e.Errors {
		if entry.Code() == code {
			return true
		}
	}
	return false
}

func (e *GraphQLError) Error() string {
	msgs := make([]string, len(e.Errors))
	for i, entry := range e.Errors {
		msgs[i] = entry.Message
	}
	return fmt.Sprintf("GraphQL error in %s: %s", e.QueryName, strings.Join(msgs, "; "))
}

// HTTPError is returned when the server replies with a status the client
// cannot interpret as a GraphQL response — most often because the request
// never reached the GraphQL handler at all.
//
// Archive-Node-API answers every GraphQL-level error with HTTP 200 and a
// populated "errors" array, so a 4xx here does not mean "bad query". The
// usual cause is a URL pointing somewhere the server does not serve: the
// endpoint is the root path "/", and "/graphql" returns 404.
//
// Body holds the raw response, truncated, which may be HTML rather than JSON.
type HTTPError struct {
	QueryName  string
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d in %s: %s", e.StatusCode, e.QueryName, e.Body)
}

// RateLimitError is returned when the server's rate limiter rejects the
// request with HTTP 429.
//
// This is the only non-200 the API emits under normal operation: every
// GraphQL-level error arrives as HTTP 200 with a populated errors array. So
// unlike most 4xx it unambiguously means "slow down", and it is the one case
// where retrying the identical request is correct.
//
// RetryAfter is zero when the server sent no usable retry-after header, and
// Limit/Remaining are -1 when their headers were absent or malformed.
type RateLimitError struct {
	QueryName  string
	RetryAfter time.Duration
	Limit      int
	Remaining  int
	Errors     []GraphQLErrorEntry
}

func (e *RateLimitError) Error() string {
	msg := "rate limited"
	if len(e.Errors) > 0 && e.Errors[0].Message != "" {
		msg = e.Errors[0].Message
	}
	if e.RetryAfter > 0 {
		return fmt.Sprintf("HTTP 429 in %s: %s (retry after %s)", e.QueryName, msg, e.RetryAfter)
	}
	return fmt.Sprintf("HTTP 429 in %s: %s", e.QueryName, msg)
}

// ConnectionError is returned when the client exhausts its retries against
// transient transport/HTTP failures.
type ConnectionError struct {
	QueryName string
	Retries   int
	LastError error
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("failed to execute %s after %d attempts: %v", e.QueryName, e.Retries, e.LastError)
}

func (e *ConnectionError) Unwrap() error {
	return e.LastError
}

// MissingFieldError is returned when a required field is absent in an
// otherwise-successful response — most likely a schema mismatch.
type MissingFieldError struct {
	QueryName string
	Field     string
}

func (e *MissingFieldError) Error() string {
	return fmt.Sprintf("missing field %q in %s response", e.Field, e.QueryName)
}

// CurrencyUnderflowError is returned when subtraction would produce a
// negative value.
type CurrencyUnderflowError struct {
	A, B Currency
}

func (e *CurrencyUnderflowError) Error() string {
	return fmt.Sprintf("currency underflow: %s - %s would be negative", e.A, e.B)
}

// InvalidCurrencyError is returned when a string can't be parsed as a
// Mina/nanomina currency value.
type InvalidCurrencyError struct {
	Input  string
	Reason string
}

func (e *InvalidCurrencyError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("invalid currency format %q: %s", e.Input, e.Reason)
	}
	return fmt.Sprintf("invalid currency format: %q", e.Input)
}

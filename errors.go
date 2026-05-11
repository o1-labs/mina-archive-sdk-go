package archive

import (
	"fmt"
	"strings"
)

// GraphQLError is returned when the server replies with a non-empty `errors`
// array. GraphQL-level errors are not retried by the client.
type GraphQLError struct {
	QueryName string
	Errors    []GraphQLErrorEntry
}

// GraphQLErrorEntry is one item from the GraphQL response's `errors` field.
type GraphQLErrorEntry struct {
	Message string `json:"message"`
}

func (e *GraphQLError) Error() string {
	msgs := make([]string, len(e.Errors))
	for i, entry := range e.Errors {
		msgs[i] = entry.Message
	}
	return fmt.Sprintf("GraphQL error in %s: %s", e.QueryName, strings.Join(msgs, "; "))
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

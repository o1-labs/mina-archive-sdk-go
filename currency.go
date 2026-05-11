package archive

import (
	"strconv"
	"strings"
)

// NanominaPerMina is the number of nanomina in one MINA.
const NanominaPerMina uint64 = 1_000_000_000

// Currency represents a Mina currency amount stored as nanomina (atomic unit).
type Currency struct {
	nanomina uint64
}

// CurrencyFromNanomina builds a Currency from a nanomina value.
func CurrencyFromNanomina(n uint64) Currency {
	return Currency{nanomina: n}
}

// CurrencyFromMina parses a decimal MINA string like "1.5", "100", or
// "0.000000001". Up to 9 decimal places. Negative or otherwise invalid
// inputs return an *InvalidCurrencyError.
func CurrencyFromMina(s string) (Currency, error) {
	n, err := parseDecimal(s)
	if err != nil {
		return Currency{}, err
	}
	return Currency{nanomina: n}, nil
}

// MustCurrencyFromMina is like CurrencyFromMina but panics on error.
// Useful for compile-time-constant values in tests/examples.
func MustCurrencyFromMina(s string) Currency {
	c, err := CurrencyFromMina(s)
	if err != nil {
		panic(err)
	}
	return c
}

// CurrencyFromGraphQL parses a nanomina-decimal string as the Archive-Node-API
// returns it in block transaction amount/fee fields.
func CurrencyFromGraphQL(s string) (Currency, error) {
	if s == "" {
		return Currency{}, &InvalidCurrencyError{Input: s}
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return Currency{}, &InvalidCurrencyError{Input: s, Reason: err.Error()}
	}
	return Currency{nanomina: n}, nil
}

// Nanomina returns the value in nanomina.
func (c Currency) Nanomina() uint64 { return c.nanomina }

// Mina returns the value as a decimal MINA string with 9 fractional digits.
func (c Currency) Mina() string {
	s := strconv.FormatUint(c.nanomina, 10)
	if len(s) > 9 {
		return s[:len(s)-9] + "." + s[len(s)-9:]
	}
	return "0." + strings.Repeat("0", 9-len(s)) + s
}

// NanominaString returns the value as a nanomina decimal string (the format
// the GraphQL API uses for amount/fee inputs).
func (c Currency) NanominaString() string {
	return strconv.FormatUint(c.nanomina, 10)
}

// String implements fmt.Stringer.
func (c Currency) String() string { return c.Mina() }

// IsZero reports whether the value is zero.
func (c Currency) IsZero() bool { return c.nanomina == 0 }

// Equal reports whether two values are equal.
func (c Currency) Equal(o Currency) bool { return c.nanomina == o.nanomina }

// Less reports c < o.
func (c Currency) Less(o Currency) bool { return c.nanomina < o.nanomina }

// LessOrEqual reports c <= o.
func (c Currency) LessOrEqual(o Currency) bool { return c.nanomina <= o.nanomina }

// Greater reports c > o.
func (c Currency) Greater(o Currency) bool { return c.nanomina > o.nanomina }

// GreaterOrEqual reports c >= o.
func (c Currency) GreaterOrEqual(o Currency) bool { return c.nanomina >= o.nanomina }

// Add returns c + o. May overflow silently — use CheckedAdd for safety.
func (c Currency) Add(o Currency) Currency {
	return Currency{nanomina: c.nanomina + o.nanomina}
}

// CheckedAdd returns c + o and reports false on uint64 overflow.
func (c Currency) CheckedAdd(o Currency) (Currency, bool) {
	sum := c.nanomina + o.nanomina
	if sum < c.nanomina {
		return Currency{}, false
	}
	return Currency{nanomina: sum}, true
}

// Sub returns c - o, or a *CurrencyUnderflowError if the result would be negative.
func (c Currency) Sub(o Currency) (Currency, error) {
	if c.nanomina < o.nanomina {
		return Currency{}, &CurrencyUnderflowError{A: c, B: o}
	}
	return Currency{nanomina: c.nanomina - o.nanomina}, nil
}

// Mul returns c * n.
func (c Currency) Mul(n uint64) Currency {
	return Currency{nanomina: c.nanomina * n}
}

func parseDecimal(s string) (uint64, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return 0, &InvalidCurrencyError{Input: s, Reason: "empty"}
	}
	if strings.HasPrefix(trimmed, "-") {
		return 0, &InvalidCurrencyError{Input: s, Reason: "negative"}
	}

	parts := strings.SplitN(trimmed, ".", 3)
	switch len(parts) {
	case 1:
		whole, err := strconv.ParseUint(parts[0], 10, 64)
		if err != nil {
			return 0, &InvalidCurrencyError{Input: s, Reason: err.Error()}
		}
		return whole * NanominaPerMina, nil
	case 2:
		left, right := parts[0], parts[1]
		if len(right) > 9 {
			return 0, &InvalidCurrencyError{Input: s, Reason: "more than 9 decimal places"}
		}
		// Allow ".5" → whole = 0
		if left == "" {
			left = "0"
		}
		combined := left + right + strings.Repeat("0", 9-len(right))
		n, err := strconv.ParseUint(combined, 10, 64)
		if err != nil {
			return 0, &InvalidCurrencyError{Input: s, Reason: err.Error()}
		}
		return n, nil
	default:
		return 0, &InvalidCurrencyError{Input: s, Reason: "multiple decimal points"}
	}
}

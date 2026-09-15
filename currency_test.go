package archive

import (
	"errors"
	"testing"
)

func TestCurrencyFromMinaInteger(t *testing.T) {
	c, err := CurrencyFromMina("5")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := c.Nanomina(), uint64(5_000_000_000); got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

func TestCurrencyFromMinaDecimal(t *testing.T) {
	c, err := CurrencyFromMina("1.5")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := c.Nanomina(), uint64(1_500_000_000); got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

func TestCurrencyFromMinaSmallestUnit(t *testing.T) {
	c, err := CurrencyFromMina("0.000000001")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := c.Nanomina(), uint64(1); got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

func TestCurrencyFromMinaNoWhole(t *testing.T) {
	c, err := CurrencyFromMina(".5")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := c.Nanomina(), uint64(500_000_000); got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

func TestCurrencyFromGraphQL(t *testing.T) {
	c, err := CurrencyFromGraphQL("1500000000")
	if err != nil {
		t.Fatal(err)
	}
	if c.Nanomina() != 1_500_000_000 {
		t.Errorf("nanomina = %d", c.Nanomina())
	}
	if c.Mina() != "1.500000000" {
		t.Errorf("mina = %q", c.Mina())
	}
}

func TestCurrencyMinaFormatting(t *testing.T) {
	cases := []struct {
		nano uint64
		want string
	}{
		{0, "0.000000000"},
		{1, "0.000000001"},
		{500_000_000, "0.500000000"},
		{1_000_000_000, "1.000000000"},
		{1_500_000_000, "1.500000000"},
		{100_000_000_000, "100.000000000"},
	}
	for _, tc := range cases {
		got := CurrencyFromNanomina(tc.nano).Mina()
		if got != tc.want {
			t.Errorf("Mina(%d) = %q, want %q", tc.nano, got, tc.want)
		}
	}
}

func TestCurrencyArithmetic(t *testing.T) {
	a := MustCurrencyFromMina("3")
	b := MustCurrencyFromMina("1")
	if sum := a.Add(b); sum.Nanomina() != 4_000_000_000 {
		t.Errorf("Add = %d", sum.Nanomina())
	}
	diff, err := a.Sub(b)
	if err != nil {
		t.Fatal(err)
	}
	if diff.Nanomina() != 2_000_000_000 {
		t.Errorf("Sub = %d", diff.Nanomina())
	}
	prod := a.Mul(3)
	if prod.Nanomina() != 9_000_000_000 {
		t.Errorf("Mul = %d", prod.Nanomina())
	}
}

func TestCurrencySubUnderflow(t *testing.T) {
	a := MustCurrencyFromMina("1")
	b := MustCurrencyFromMina("2")
	_, err := a.Sub(b)
	var underflow *CurrencyUnderflowError
	if !errors.As(err, &underflow) {
		t.Errorf("expected CurrencyUnderflowError, got %v", err)
	}
}

func TestCurrencyCheckedAddOverflow(t *testing.T) {
	max := CurrencyFromNanomina(^uint64(0))
	one := CurrencyFromNanomina(1)
	if _, ok := max.CheckedAdd(one); ok {
		t.Errorf("expected overflow to return false")
	}
}

func TestCurrencyCompare(t *testing.T) {
	a := MustCurrencyFromMina("1")
	b := MustCurrencyFromMina("2")
	if !a.Less(b) || !b.Greater(a) {
		t.Error("compare failed")
	}
	if !a.Equal(a) {
		t.Error("equal failed")
	}
}

func TestCurrencyRejectsBadInput(t *testing.T) {
	cases := []string{"abc", "", "-1", "1.0000000001", "1.2.3"}
	for _, in := range cases {
		if _, err := CurrencyFromMina(in); err == nil {
			t.Errorf("expected error for %q", in)
		}
	}
}

func TestCurrencyFromGraphQLRejectsBadInput(t *testing.T) {
	cases := []string{"", "abc", "-5", "1.5"}
	for _, in := range cases {
		if _, err := CurrencyFromGraphQL(in); err == nil {
			t.Errorf("expected error for %q", in)
		}
	}
}

func TestNanominaString(t *testing.T) {
	c := MustCurrencyFromMina("3")
	if c.NanominaString() != "3000000000" {
		t.Errorf("got %q", c.NanominaString())
	}
}

// CurrencyFromMina is documented to reject invalid input, but the no-decimal
// branch multiplied without an overflow check (#10). "18446744074" wrapped to
// 0.29 MINA and reported success, while a value one digit longer was correctly
// rejected — so the failure was not even uniform.
func TestCurrencyFromMinaOverflowBoundary(t *testing.T) {
	cases := []struct {
		in       string
		wantErr  bool
		wantNano uint64
	}{
		{"0", false, 0},
		{"1", false, 1_000_000_000},
		// math.MaxUint64 / NanominaPerMina — the last value that fits.
		{"18446744073", false, 18446744073_000000000},
		// One more wrapped to 290448384 and reported success.
		{"18446744074", true, 0},
		{"18446744073709551615", true, 0},
		{"99999999999999999999", true, 0},
	}

	for _, tc := range cases {
		got, err := CurrencyFromMina(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("CurrencyFromMina(%q) = %d, want an error", tc.in, got.Nanomina())
				continue
			}
			var invErr *InvalidCurrencyError
			if !errors.As(err, &invErr) {
				t.Errorf("CurrencyFromMina(%q) error = %T, want *InvalidCurrencyError", tc.in, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("CurrencyFromMina(%q) unexpected error: %v", tc.in, err)
			continue
		}
		if got.Nanomina() != tc.wantNano {
			t.Errorf("CurrencyFromMina(%q) = %d, want %d", tc.in, got.Nanomina(), tc.wantNano)
		}
	}
}

// The decimal branch was already safe; assert it stays that way and that the
// boundary behaves identically with an explicit fractional part.
func TestCurrencyFromMinaDecimalBoundary(t *testing.T) {
	if _, err := CurrencyFromMina("18446744073.709551615"); err != nil {
		t.Errorf("largest representable value should parse: %v", err)
	}
	if _, err := CurrencyFromMina("18446744073.709551616"); err == nil {
		t.Error("one nanomina past the maximum should fail")
	}
}

func TestCheckedMul(t *testing.T) {
	max := CurrencyFromNanomina(^uint64(0))
	if _, ok := max.CheckedMul(2); ok {
		t.Error("CheckedMul(2) on MaxUint64 = ok, want overflow reported")
	}
	if c, ok := max.CheckedMul(1); !ok || c.Nanomina() != ^uint64(0) {
		t.Errorf("CheckedMul(1) = (%d, %v), want (MaxUint64, true)", c.Nanomina(), ok)
	}
	if c, ok := max.CheckedMul(0); !ok || c.Nanomina() != 0 {
		t.Errorf("CheckedMul(0) = (%d, %v), want (0, true)", c.Nanomina(), ok)
	}
	if c, ok := CurrencyFromNanomina(0).CheckedMul(^uint64(0)); !ok || c.Nanomina() != 0 {
		t.Errorf("zero.CheckedMul(max) = (%d, %v), want (0, true)", c.Nanomina(), ok)
	}
	if c, ok := CurrencyFromNanomina(3).CheckedMul(4); !ok || c.Nanomina() != 12 {
		t.Errorf("3.CheckedMul(4) = (%d, %v), want (12, true)", c.Nanomina(), ok)
	}
}

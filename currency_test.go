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

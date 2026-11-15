package money

import (
	"fmt"
	"math"
	"strconv"
	"testing"
)

func TestParseEURCents(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  int
		ok    bool
	}{
		{"", 0, true}, {"0", 0, true}, {"12", 1200, true}, {"1.", 100, true},
		{"12.5", 1250, true}, {"12.50", 1250, true}, {".50", 50, true},
		{"-1", 0, false}, {"-0", 0, false}, {"-0.50", 0, false}, {"-0.01", 0, false},
		{"1.-1", 0, false},
		{"NaN", 0, false}, {"Inf", 0, false}, {".", 0, false},
		{"1.-0", 0, false}, {"1.+1", 0, false},
		{"1.001", 0, false}, {"1.2.3", 0, false}, {"nope", 0, false},
		{"999999999999999999999999", 0, false},
	} {
		got, err := ParseEURCents(tc.value)
		if (err == nil) != tc.ok || got != tc.want {
			t.Fatalf("ParseEURCents(%q) = %d, %v", tc.value, got, err)
		}
	}
}

func TestFormatEURCentsAvoidsFloatPrecisionLoss(t *testing.T) {
	if got, want := FormatEURCents(math.MaxInt), fmt.Sprintf("%d.%02d€", math.MaxInt/100, math.MaxInt%100); got != want {
		t.Fatalf("FormatEURCents(max int) = %q", got)
	}
	if got := FormatEURCents(-50); got != "-0.50€" {
		t.Fatalf("FormatEURCents(-50) = %q", got)
	}
}

func TestArithmeticValidatesAmountsAndOverflow(t *testing.T) {
	if got, err := MultiplyCents(450, 2); err != nil || got != 900 {
		t.Fatalf("MultiplyCents = %d, %v", got, err)
	}
	if _, err := MultiplyCents(1, -1); err == nil {
		t.Fatal("expected negative quantity rejection")
	}
	if _, err := MultiplyCents(2, math.MaxInt); err == nil {
		t.Fatal("expected multiplication overflow")
	}
	if _, err := MultiplyCents(-1, 1); err == nil {
		t.Fatal("expected negative price rejection")
	}
	if _, err := AddCents(math.MaxInt, 1); err == nil {
		t.Fatal("expected addition overflow")
	}
	if got, err := AddCents(1, 2); err != nil || got != 3 {
		t.Fatalf("AddCents = %d, %v", got, err)
	}
	if _, err := AddCents(-1, 1); err == nil {
		t.Fatal("expected negative amount rejection")
	}
}

func TestParseEURCentsRejectsIntegerThatOverflowsCents(t *testing.T) {
	if _, err := ParseEURCents(strconv.Itoa(math.MaxInt)); err == nil {
		t.Fatal("expected amount overflow")
	}
}

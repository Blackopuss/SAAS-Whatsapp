package web

import (
	"reflect"
	"testing"
)

func TestParsePrice(t *testing.T) {
	ok := map[string]int32{"289": 28900, "289.5": 28950, "289.50": 28950, "$1,289.00": 128900, " 0.99 ": 99, ".5": 50, "1000000": 100000000}
	for in, want := range ok {
		got, err := parsePrice(in)
		if err != nil || got != want {
			t.Errorf("parsePrice(%q) = %d, %v; quiero %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "abc", "-5", "1.234", "12.", "1e3", "1000000.01", "99999999"} {
		if _, err := parsePrice(in); err == nil {
			t.Errorf("parsePrice(%q) debió fallar", in)
		}
	}
}

func TestFormatPrice(t *testing.T) {
	for cents, want := range map[int32]string{28900: "289", 28950: "289.50", 99: "0.99", 105: "1.05"} {
		if got := formatPrice(cents); got != want {
			t.Errorf("formatPrice(%d) = %q, quiero %q", cents, got, want)
		}
	}
}

func TestSplitListAndMatrix(t *testing.T) {
	sizes := splitList(" S, m ,M, L|,, ", 20)
	if !reflect.DeepEqual(sizes, []string{"S", "m", "L"}) {
		t.Fatalf("sizes = %q", sizes)
	}
	if got := variantMatrix(sizes, []string{"Negro", "Blanco"}); len(got) != 6 || got[0] != (variantKey{"S", "Negro"}) {
		t.Fatalf("matrix = %v", got)
	}
	if got := variantMatrix(nil, []string{"Rojo"}); len(got) != 1 || got[0] != (variantKey{"", "Rojo"}) {
		t.Fatalf("solo color = %v", got)
	}
	if variantMatrix(nil, nil) != nil {
		t.Fatal("sin tallas ni colores no hay variantes")
	}
}

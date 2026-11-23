package billing

import (
	"strings"
	"testing"
	"time"
)

func d(s string) time.Time { t, _ := time.Parse("2006-01-02", s); return t }

func TestApply(t *testing.T) {
	// Primer pago: un mes desde hoy, racha 1.
	until, streak, onTime := Apply(nil, 0, d("2026-11-22"), 1)
	if until != d("2026-12-22") || streak != 1 || !onTime {
		t.Errorf("primer pago: %v %d %v", until, streak, onTime)
	}
	// A tiempo (antes de vencer): suma desde el vencimiento, no desde hoy; la racha crece.
	prev := d("2026-12-22")
	until, streak, onTime = Apply(&prev, 3, d("2026-12-20"), 2)
	if until != d("2027-02-22") || streak != 5 || !onTime {
		t.Errorf("a tiempo: %v %d %v", until, streak, onTime)
	}
	// El mismo día del vencimiento cuenta como a tiempo.
	if _, _, onTime = Apply(&prev, 3, prev, 1); !onTime {
		t.Error("pagar el día del vencimiento debe contar a tiempo")
	}
	// Tarde: cuenta desde el pago y la racha reinicia.
	until, streak, onTime = Apply(&prev, 7, d("2026-12-30"), 1)
	if until != d("2027-01-30") || streak != 1 || onTime {
		t.Errorf("tarde: %v %d %v", until, streak, onTime)
	}
}

func TestNotice(t *testing.T) {
	today := d("2026-11-22")
	at := func(s string) *time.Time { x := d(s); return &x }
	for _, c := range []struct {
		status string
		until  *time.Time
		want   string
	}{
		{"active", nil, ""},
		{"active", at("2026-12-01"), ""},
		{"active", at("2026-11-25"), "en 3 días"},
		{"active", at("2026-11-23"), "en 1 día)"},
		{"active", at("2026-11-22"), "vence hoy"},
		{"active", at("2026-11-20"), "se pausará el 26 de noviembre de 2026"},
		{"suspended", at("2026-11-01"), "en pausa"},
	} {
		got := Notice(c.status, c.until, today)
		if (c.want == "" && got != "") || !strings.Contains(got, c.want) {
			t.Errorf("%s %v: %q, quiero %q", c.status, c.until, got, c.want)
		}
	}
}

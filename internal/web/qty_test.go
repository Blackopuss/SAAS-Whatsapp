package web

import "testing"

func TestParseQty(t *testing.T) {
	for in, want := range map[string]int32{"2": 2000, "0.25": 250, "1,5": 1500, ".5": 500, "10.125": 10125} {
		got, ok, err := parseQty(in)
		if err != nil || !ok || got != want {
			t.Errorf("parseQty(%q) = %d, %v, %v; quiero %d", in, got, ok, err, want)
		}
		if back := formatQty(got); back != map[string]string{"2": "2", "0.25": "0.25", "1,5": "1.5", ".5": "0.5", "10.125": "10.125"}[in] {
			t.Errorf("formatQty(%d) = %q", got, back)
		}
	}
	if _, ok, err := parseQty(""); ok || err != nil {
		t.Error("vacío debe ser sin valor y sin error")
	}
	for _, bad := range []string{"1.2345", "-1", "abc", "1.", "99999999"} {
		if _, _, err := parseQty(bad); err == nil {
			t.Errorf("parseQty(%q) debió fallar", bad)
		}
	}
}

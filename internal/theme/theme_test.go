package theme

import "testing"

func TestFit(t *testing.T) {
	for _, hex := range []string{"#ff69b4", "#ffff00", "#25d366", "#7f7f7f", "#000000", "#ffffff", "#c2185b", "#1e3a8a", "#ff8c00", "#00ffff"} {
		brand, ink, adjusted, err := Fit(hex)
		if err != nil {
			t.Fatal(err)
		}
		if got := contrast(mustParse(brand), mustParse(ink)); got < minRate {
			t.Errorf("%s -> %s con %s: contraste %.2f", hex, brand, ink, got)
		}
		if !adjusted && brand != hex {
			t.Errorf("%s cambió a %s sin marcar ajuste", hex, brand)
		}
		if InkFor(brand) != ink {
			t.Errorf("%s: InkFor = %s, Fit = %s", brand, InkFor(brand), ink)
		}
	}
	if _, _, adjusted, _ := Fit("#1e3a8a"); adjusted {
		t.Error("azul marino no necesitaba ajuste")
	}
	if _, _, adjusted, _ := Fit("#7f7f7f"); !adjusted {
		t.Error("gris medio sí necesitaba ajuste")
	}
	for _, bad := range []string{"", "red", "#12345", "#gggggg", "javascript:"} {
		if _, _, _, err := Fit(bad); err == nil {
			t.Errorf("%q aceptado", bad)
		}
	}
}

func TestOKLCHRoundTrip(t *testing.T) {
	for _, hex := range []string{"#c2185b", "#25d366", "#1e3a8a", "#e7d8c0"} {
		l, c, h := toOKLCH(mustParse(hex))
		if got := format(fromOKLCH(l, c, h)); got != hex {
			t.Errorf("%s -> %s", hex, got)
		}
	}
}

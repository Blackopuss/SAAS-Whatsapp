package web

import (
	"testing"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
)

func TestNormalizeWhatsapp(t *testing.T) {
	for in, want := range map[string]string{
		"55 1234 5678":     "+525512345678",
		"(55) 1234-5678":   "+525512345678",
		"+52 55 1234 5678": "+525512345678",
		"525512345678":     "+525512345678",
		"+1 415 555 0100":  "+14155550100",
		"":                 "",
	} {
		if got, ok := normalizeWhatsapp(in); !ok || got != want {
			t.Errorf("%q = %q %v, quiero %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"123", "+0 55 1234 5678", "abc", "+52 55 1234 5678 9999 99"} {
		if got, ok := normalizeWhatsapp(in); ok {
			t.Errorf("%q aceptado como %q", in, got)
		}
	}
}

func TestSalePrice(t *testing.T) {
	cases := []struct {
		price, sale    int32
		onSale, valid  bool
		wantNow, wantW int32
	}{
		{28900, 24900, true, true, 24900, 28900},
		{28900, 24900, false, true, 28900, 0}, // precio de oferta guardado pero sin marcar
		{28900, 0, true, false, 28900, 0},     // marcada sin precio de oferta
		{28900, 30000, true, true, 28900, 0},  // "oferta" más cara no cuenta
	}
	for _, c := range cases {
		if now, was := salePrice(c.price, c.onSale, c.sale, c.valid); now != c.wantNow || was != c.wantW {
			t.Errorf("%+v: %d %d", c, now, was)
		}
	}
}

func TestOptions(t *testing.T) {
	vs := []db.ProductVariant{
		{Size: "S", Color: "Blanco", SoldOut: true}, {Size: "M", Color: "Blanco"},
		{Size: "S", Color: "Negro", SoldOut: true}, {Size: "M", Color: "Negro", SoldOut: true},
	}
	sizes, colors, sold := options(vs)
	if len(sizes) != 2 || sizes[0].Name != "S" || !sizes[0].SoldOut || sizes[1].SoldOut {
		t.Errorf("tallas: %+v", sizes)
	}
	if len(colors) != 2 || colors[0].SoldOut || !colors[1].SoldOut || colors[1].Swatch != "#1c1917" {
		t.Errorf("colores: %+v", colors)
	}
	if len(sold) != 3 || sold[0] != "S|Blanco" {
		t.Errorf("agotadas: %v", sold)
	}
}

func TestParseFAQ(t *testing.T) {
	got := parseFAQ("¿Hacen envíos?\nSí, a todo México.\n\n\n¿Apartan?\n\nSolo pregunta sin respuesta")
	if len(got) != 3 || got[0].Q != "¿Hacen envíos?" || got[0].A != "Sí, a todo México." || got[1].A != "" || got[2].Q != "Solo pregunta sin respuesta" {
		t.Fatalf("%+v", got)
	}
	if len(parseFAQ("  \n\n ")) != 0 {
		t.Fatal("texto vacío dio preguntas")
	}
}

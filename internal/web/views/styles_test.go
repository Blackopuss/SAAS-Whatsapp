package views

import (
	"testing"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/theme"
)

// Los colores sugeridos deben pasar contraste sin ajuste, o la vendedora vería el aviso de "ajustamos tu color".
func TestStyleBrandsPassContrast(t *testing.T) {
	for _, s := range Styles {
		if _, _, adjusted, err := theme.Fit(s.Brand); err != nil || adjusted {
			t.Errorf("%s: color %s ajustado=%v err=%v", s.ID, s.Brand, adjusted, err)
		}
	}
	if StyleByID("nada").ID != "vitrina" || ValidStyle("nada") || !ValidStyle("barberia") {
		t.Error("StyleByID/ValidStyle")
	}
}

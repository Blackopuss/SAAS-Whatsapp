package web

import (
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"rsc.io/qr"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/theme"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/web/views"
)

func (a *app) shopURL(r *http.Request, slug string) string {
	return strings.TrimSuffix(a.resetLink(r.Host, slug, ""), "/panel/recuperar?t=") + "/"
}

// abs vuelve absoluta una URL de foto (en desarrollo es "/media/..."); Open Graph exige URLs completas.
func (a *app) abs(r *http.Request, slug, u string) string {
	if u == "" || strings.HasPrefix(u, "http") {
		return u
	}
	return strings.TrimSuffix(a.shopURL(r, slug), "/") + u
}

func (a *app) myStoreForm(r *http.Request, s db.Store) views.StoreForm {
	color := s.BrandColorRaw
	if color == "" {
		color = views.StyleByID(s.Style).Brand
	}
	return views.StoreForm{
		Name: s.Name, ShopURL: a.shopURL(r, s.Slug), Whatsapp: s.Whatsapp, Color: color, Applied: s.BrandColor,
		FAQ: s.Faq, Payment: s.PaymentInfo, Logo: a.photo(s.LogoKey, s.LogoWidths), Style: s.Style,
	}
}

func (a *app) myStore(w http.ResponseWriter, r *http.Request) {
	s, err := a.q.GetStoreBySlug(r.Context(), StoreFrom(r.Context()).Slug) // sin caché: muestra lo recién guardado
	if err != nil {
		a.fail(w, r, err)
		return
	}
	f := a.myStoreForm(r, s)
	q := r.URL.Query()
	f.Saved = q.Has("guardado")
	f.Adjusted = f.Saved && s.BrandColorRaw != "" && s.BrandColorRaw != s.BrandColor
	f.LogoError = photoErrors[q.Get("logo")]
	f.LogoOK = q.Get("logo") == "ok"
	renderStatus(w, r, http.StatusOK, views.MyStore(f))
}

func (a *app) myStoreSave(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s, err := a.q.GetStoreBySlug(ctx, StoreFrom(ctx).Slug)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	f := a.myStoreForm(r, s)
	f.Name = strings.TrimSpace(r.PostFormValue("name"))
	f.Whatsapp = r.PostFormValue("whatsapp")
	f.Color = strings.ToLower(strings.TrimSpace(r.PostFormValue("color")))
	f.FAQ = strings.TrimSpace(strings.ReplaceAll(r.PostFormValue("faq"), "\r\n", "\n"))
	f.Payment = strings.TrimSpace(strings.ReplaceAll(r.PostFormValue("payment"), "\r\n", "\n"))
	f.Style = r.PostFormValue("style")

	wa, waOK := normalizeWhatsapp(f.Whatsapp)
	brand, _, _, colorErr := theme.Fit(f.Color)
	switch {
	case f.Name == "" || utf8.RuneCountInString(f.Name) > 80:
		f.Error = "Escribe el nombre de tu tienda (máximo 80 letras)."
	case !waOK || len(f.Whatsapp) > 40:
		f.Error = "Revisa el WhatsApp: deben ser 10 dígitos si es de México, o empezar con + y la lada si es de otro país."
	case colorErr != nil:
		f.Error = "Elige un color de la paleta."
	case utf8.RuneCountInString(f.FAQ) > 4000:
		f.Error = "Las preguntas frecuentes pueden tener hasta 4000 letras."
	case utf8.RuneCountInString(f.Payment) > 1000:
		f.Error = "Los datos de pago pueden tener hasta 1000 letras."
	case !views.ValidStyle(f.Style):
		f.Error = "Elige un estilo para tu tienda."
	}
	if f.Error != "" {
		renderStatus(w, r, http.StatusUnprocessableEntity, views.MyStore(f))
		return
	}
	err = a.tx(ctx, func(q db.Querier) error {
		if err := q.UpdateStoreWhatsapp(ctx, db.UpdateStoreWhatsappParams{ID: s.ID, Whatsapp: wa}); err != nil {
			return err
		}
		return q.UpdateStoreSettings(ctx, db.UpdateStoreSettingsParams{
			ID: s.ID, Name: f.Name, BrandColor: brand, BrandColorRaw: f.Color, Faq: f.FAQ, PaymentInfo: f.Payment, Style: f.Style,
		})
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.tenants.forget(s.Slug)
	back(w, r, "/panel/tienda?guardado")
}

// myStorePreview pinta la tienda con los valores aún sin guardar, para el <iframe> de "Mi tienda".
func (a *app) myStorePreview(w http.ResponseWriter, r *http.Request) {
	// Única página que se deja enmarcar, y solo por el propio panel.
	h := w.Header()
	h.Set("Content-Security-Policy", strings.Replace(h.Get("Content-Security-Policy"), "frame-ancestors 'none'", "frame-ancestors 'self'", 1))
	sh, ps, err := a.shop(r, 0)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	q := r.URL.Query()
	if n := strings.TrimSpace(q.Get("nombre")); n != "" && utf8.RuneCountInString(n) <= 80 {
		sh.Name = n
	}
	if brand, ink, _, err := theme.Fit(strings.ToLower(q.Get("color"))); err == nil {
		sh.Brand, sh.BrandInk = brand, ink
	}
	if st := q.Get("estilo"); views.ValidStyle(st) {
		sh.Style = st
	}
	sh.Preview = true
	renderStatus(w, r, http.StatusOK, views.ShopHome(sh, a.cards(ps, 0)))
}

func (a *app) myStoreQR(w http.ResponseWriter, r *http.Request) {
	s := StoreFrom(r.Context())
	code, err := qr.Encode(a.shopURL(r, s.Slug), qr.M)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	code.Scale = 12
	w.Header().Set("Content-Type", "image/png")
	if r.URL.Query().Has("descargar") {
		w.Header().Set("Content-Disposition", `attachment; filename="qr-`+url.PathEscape(s.Slug)+`.png"`)
	}
	_, _ = w.Write(code.PNG()) // #nosec G705 -- PNG binario con Content-Type image/png, no HTML
}

func (a *app) logoUpload(w http.ResponseWriter, r *http.Request) {
	ctx, s := r.Context(), StoreFrom(r.Context())
	up, code, err := a.saveUpload(r, "logo", s.ID, 1)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if code != "" {
		back(w, r, "/panel/tienda?logo="+url.QueryEscape(code)+"#logo")
		return
	}
	old, err := a.q.SetStoreLogo(ctx, db.SetStoreLogoParams{ID: s.ID, LogoKey: up.base, LogoWidths: up.widths})
	if err != nil {
		a.cleanup(ctx, up.keys)
		a.fail(w, r, err)
		return
	}
	a.cleanup(ctx, photoKeys(old.LogoKey, old.LogoWidths))
	a.tenants.forget(s.Slug)
	back(w, r, "/panel/tienda?logo=ok#logo")
}

func (a *app) logoDelete(w http.ResponseWriter, r *http.Request) {
	ctx, s := r.Context(), StoreFrom(r.Context())
	old, err := a.q.SetStoreLogo(ctx, db.SetStoreLogoParams{ID: s.ID, LogoKey: "", LogoWidths: []int32{}})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.cleanup(ctx, photoKeys(old.LogoKey, old.LogoWidths))
	a.tenants.forget(s.Slug)
	back(w, r, "/panel/tienda#logo")
}

// parseFAQ: cada bloque separado por una línea vacía; la primera línea es la pregunta y el resto la respuesta.
func parseFAQ(text string) []views.QA {
	var out []views.QA
	for _, block := range strings.Split(text, "\n\n") {
		q, a, _ := strings.Cut(strings.TrimSpace(block), "\n")
		if q = strings.TrimSpace(q); q != "" {
			out = append(out, views.QA{Q: q, A: strings.TrimSpace(a)})
		}
	}
	return out
}

// messages: plantillas de WhatsApp para que la vendedora atienda cada etapa (dudas, confirmación, recompra...).
// Los textos viven en panel.tsx; aquí solo van el nombre y la liga de la tienda.
func (a *app) messages(w http.ResponseWriter, r *http.Request) {
	s := StoreFrom(r.Context())
	renderStatus(w, r, http.StatusOK, views.Messages(s.Name, a.shopURL(r, s.Slug), s.PaymentInfo))
}

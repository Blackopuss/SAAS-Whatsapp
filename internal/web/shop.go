package web

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/storage"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/theme"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/web/views"
)

// photo arma src/srcset de una foto ya procesada.
func (a *app) photo(key string, widths []int32) views.Photo {
	var p views.Photo
	for _, w := range widths {
		p.SrcSet += a.mediaURL + storage.VariantKey(key, w, "webp") + " " + strconv.Itoa(int(w)) + "w, "
	}
	if len(widths) > 0 {
		p.Src = a.mediaURL + storage.VariantKey(key, widths[0], "webp")
		p.OG = a.mediaURL + storage.OGKey(key)
	}
	return p
}

// salePrice: la oferta solo cuenta si está marcada y es menor al precio normal.
func salePrice(price int32, onSale bool, sale int32, saleValid bool) (now, was int32) {
	if onSale && saleValid && sale < price {
		return sale, price
	}
	return price, 0
}

func (a *app) shop(r *http.Request, current int64) (views.Shop, []db.ListStoreProductsRow, error) {
	ctx := r.Context()
	s := StoreFrom(ctx)
	cats, err := a.q.ListCategories(ctx, s.ID)
	if err != nil {
		return views.Shop{}, nil, err
	}
	ps, err := a.q.ListStoreProducts(ctx, s.ID)
	if err != nil {
		return views.Shop{}, nil, err
	}
	// Solo categorías con algo que ver.
	used := map[int64]bool{}
	for _, p := range ps {
		used[p.CategoryID.Int64] = true
	}
	sh := views.Shop{
		Name: s.Name, Current: current, Whatsapp: strings.TrimPrefix(s.Whatsapp, "+"),
		Logo: a.photo(s.LogoKey, s.LogoWidths), FAQ: parseFAQ(s.Faq), Payment: s.PaymentInfo,
		URL: a.shopURL(r, s.Slug), OGImage: a.abs(r, s.Slug, a.photo(s.LogoKey, s.LogoWidths).OG),
		Style: s.Style,
	}
	if theme.Valid(s.BrandColor) {
		sh.Brand, sh.BrandInk = s.BrandColor, theme.InkFor(s.BrandColor)
	} else if b, ink, _, err := theme.Fit(views.StyleByID(s.Style).Brand); err == nil {
		sh.Brand, sh.BrandInk = b, ink // sin color propio: el del estilo
	}
	for _, c := range cats {
		if used[c.ID] {
			sh.Categories = append(sh.Categories, c)
		}
	}
	return sh, ps, nil
}

func (a *app) cards(ps []db.ListStoreProductsRow, category int64) []views.Card {
	var out []views.Card
	for _, p := range ps {
		if category != 0 && p.CategoryID.Int64 != category {
			continue
		}
		now, was := salePrice(p.PriceCents, p.OnSale, p.SalePriceCents.Int32, p.SalePriceCents.Valid)
		out = append(out, views.Card{
			ID: p.ID, Name: p.Name, Price: now, Was: was, SoldOut: p.SoldOut, New: p.IsNew,
			Photo: a.photo(p.ImageKey, p.ImageWidths), Color: p.ImageColor, Sizes: p.Sizes, CategoryID: p.CategoryID.Int64,
			Unit: units[p.Unit], Step: p.Step, Available: p.Available, Colors: colorOpts(p.Colors),
		})
	}
	return out
}

func (a *app) shopHome(w http.ResponseWriter, r *http.Request) {
	sh, ps, err := a.shop(r, 0)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	cards := a.cards(ps, 0)
	if sh.OGImage == "" && len(cards) > 0 {
		sh.OGImage = a.abs(r, StoreFrom(r.Context()).Slug, cards[0].Photo.OG)
	}
	renderStatus(w, r, http.StatusOK, views.ShopHome(sh, cards))
}

func (a *app) shopCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	sh, ps, err := a.shop(r, id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	for _, c := range sh.Categories {
		if c.ID == id {
			renderStatus(w, r, http.StatusOK, views.ShopCategory(sh, c.Name, a.cards(ps, id)))
			return
		}
	}
	http.NotFound(w, r)
}

func (a *app) shopProduct(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	s := StoreFrom(ctx)
	p, err := a.q.GetProduct(ctx, db.GetProductParams{StoreID: s.ID, ID: id})
	if isNoRows(err) || (err == nil && !p.Active) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	sh, ps, err := a.shop(r, p.CategoryID.Int64)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	imgs, err := a.q.ListProductImages(ctx, db.ListProductImagesParams{StoreID: s.ID, ProductID: id})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	vs, err := a.q.ListVariants(ctx, db.ListVariantsParams{StoreID: s.ID, ProductID: id})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	now, was := salePrice(p.PriceCents, p.OnSale, p.SalePriceCents.Int32, p.SalePriceCents.Valid)
	pp := views.ProductPage{
		Card:        views.Card{ID: p.ID, Name: p.Name, Price: now, Was: was, SoldOut: p.SoldOut, New: p.IsNew, Unit: units[p.Unit], Step: p.Step},
		Description: p.Description,
	}
	for _, x := range ps { // agotado ya calculado con existencias (ListStoreProducts)
		if x.ID == p.ID {
			pp.SoldOut = x.SoldOut
		}
	}
	for i, v := range vs { // una combinación sin existencia suficiente cuenta como agotada
		vs[i].SoldOut = v.SoldOut || (v.Stock.Valid && v.Stock.Int32 < p.Step)
	}
	for _, im := range imgs {
		pp.Photos = append(pp.Photos, a.photo(im.R2Key, im.Widths))
		if pp.Color == "" {
			pp.Color = im.DominantColor
		}
	}
	for _, c := range sh.Categories {
		if c.ID == p.CategoryID.Int64 {
			pp.CategoryID, pp.CategoryName = c.ID, c.Name
		}
	}
	pp.Sizes, pp.Colors, pp.SoldOutCombos = options(vs)
	pp.Related = related(a.cards(ps, 0), p.ID, p.CategoryID.Int64, ps)
	sh.URL += "p/" + strconv.FormatInt(p.ID, 10)
	if len(imgs) > 0 && len(imgs[0].Widths) > 0 { // la versión más grande: WhatsApp y Facebook la recortan a su gusto
		im := imgs[0]
		sh.OGImage = a.abs(r, s.Slug, a.mediaURL+storage.OGKey(im.R2Key))
	}
	renderStatus(w, r, http.StatusOK, views.ShopProduct(sh, pp))
}

// related: hasta 6 disponibles de la misma categoría y, si faltan, del resto de la tienda.
func related(cards []views.Card, self, category int64, ps []db.ListStoreProductsRow) []views.Card {
	cat := make(map[int64]int64, len(ps))
	for _, p := range ps {
		cat[p.ID] = p.CategoryID.Int64
	}
	var same, other []views.Card
	for _, c := range cards {
		switch {
		case c.ID == self || c.SoldOut:
		case cat[c.ID] == category:
			same = append(same, c)
		default:
			other = append(other, c)
		}
	}
	out := append(same, other...)
	return out[:min(6, len(out))]
}

// options agrupa variantes en tallas y colores; una opción se marca agotada si todas sus combinaciones lo están.
func options(vs []db.ProductVariant) (sizes, colors []views.Opt, soldOut []string) {
	idx := map[string]int{}
	add := func(list *[]views.Opt, prefix, name string, sold bool) {
		if name == "" {
			return
		}
		k := prefix + name
		if i, ok := idx[k]; ok {
			(*list)[i].SoldOut = (*list)[i].SoldOut && sold
			return
		}
		idx[k] = len(*list)
		*list = append(*list, views.Opt{Name: name, SoldOut: sold, Swatch: swatches[strings.ToLower(name)]})
	}
	for _, v := range vs {
		add(&sizes, "s:", v.Size, v.SoldOut)
		add(&colors, "c:", v.Color, v.SoldOut)
		if v.SoldOut {
			soldOut = append(soldOut, variantKey{v.Size, v.Color}.String())
		}
	}
	for i := range sizes {
		sizes[i].Swatch = ""
	}
	return sizes, colors, soldOut
}

// swatches: el círculo de color solo aparece si conocemos el nombre; siempre se muestra el nombre.
var swatches = map[string]string{
	"blanco": "#ffffff", "negro": "#1c1917", "gris": "#9ca3af", "rojo": "#dc2626", "vino": "#7f1d1d",
	"rosa": "#f472b6", "fucsia": "#d946ef", "morado": "#7e22ce", "lila": "#c4b5fd", "azul": "#2563eb",
	"marino": "#1e3a8a", "azul marino": "#1e3a8a", "celeste": "#7dd3fc", "verde": "#16a34a", "olivo": "#65a30d",
	"amarillo": "#facc15", "mostaza": "#ca8a04", "naranja": "#f97316", "café": "#78350f", "cafe": "#78350f",
	"beige": "#e7d8c0", "menta": "#a8dcc7", "crema": "#f5efe0", "arena": "#d6c3a1", "dorado": "#d4a017", "plateado": "#c0c0c0",
	"mezclilla": "#3b5b8a", "coral": "#fb7185", "turquesa": "#14b8a6", "camel": "#c19a6b",
}

func (a *app) shopFAQ(w http.ResponseWriter, r *http.Request) {
	sh, _, err := a.shop(r, 0)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	renderStatus(w, r, http.StatusOK, views.ShopFAQ(sh))
}

type cartProduct struct {
	Name    string           `json:"n"`
	Price   int32            `json:"p"`
	URL     string           `json:"u"`
	SoldOut bool             `json:"x"`
	Image   string           `json:"i"`
	Unit    string           `json:"un"`
	Step    int32            `json:"t"` // milésimas
	Stock   int32            `json:"s"` // disponible en milésimas; -1 = no lleva inventario
	Combos  map[string]int32 `json:"v"` // "talla|color" -> disponible (milésimas, -1 sin inventario, 0 agotada)
}

// shopCartJSON: el carrito valida precios y disponibilidad contra esto, nunca contra lo guardado en el celular.
func (a *app) shopCartJSON(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := StoreFrom(ctx)
	ps, err := a.q.ListStoreProducts(ctx, s.ID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	vs, err := a.q.ListStoreVariants(ctx, s.ID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	out := make(map[string]*cartProduct, len(ps))
	for _, p := range ps {
		now, _ := salePrice(p.PriceCents, p.OnSale, p.SalePriceCents.Int32, p.SalePriceCents.Valid)
		id := strconv.FormatInt(p.ID, 10)
		out[id] = &cartProduct{Name: p.Name, Price: now, URL: "/p/" + id, SoldOut: p.SoldOut, Image: a.photo(p.ImageKey, p.ImageWidths).Src,
			Unit: units[p.Unit], Step: p.Step, Stock: publicStock(available(p.SoldOut, p.Stock), p.Step), Combos: map[string]int32{}}
	}
	for _, v := range vs {
		if p := out[strconv.FormatInt(v.ProductID, 10)]; p != nil {
			p.Combos[variantKey{v.Size, v.Color}.String()] = publicStock(available(v.SoldOut, v.Stock), p.Step)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		Log(ctx).Error("carrito.json", "err", err)
	}
}

var e164 = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

// normalizeWhatsapp acepta "55 1234 5678", "+52 55..." o "5255..."; 10 dígitos se toman como México.
func normalizeWhatsapp(in string) (string, bool) {
	in = strings.TrimSpace(in)
	if in == "" {
		return "", true
	}
	d := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, in)
	switch {
	case strings.HasPrefix(in, "+"):
		d = "+" + d
	case len(d) == 10:
		d = "+52" + d
	default:
		d = "+" + d
	}
	return d, e164.MatchString(d)
}

// requireOpen: una tienda en pausa no muestra nada de la vendedora (ni nombre, ni WhatsApp, ni productos).
func (a *app) requireOpen(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if StoreFrom(r.Context()).Status != "active" {
			w.Header().Set("Retry-After", "86400")
			renderStatus(w, r, http.StatusServiceUnavailable, views.Paused())
			return
		}
		next.ServeHTTP(w, r)
	})
}

// publicStock: lo que /carrito.json dice de la existencia. Hasta 99 pasos es exacto (el carrito no deja
// pedir más); arriba se recorta, para que la competencia no lea el inventario real de la tienda.
func publicStock(avail, step int32) int32 {
	if limit := step * maxUnits; avail > limit {
		return limit
	}
	return avail
}

const maxUnits = 99 // igual que MAX_UNITS en shop.tsx

// colorOpts: los colores de una tarjeta con su muestra (si el nombre es uno conocido).
func colorOpts(names []string) []views.Opt {
	out := make([]views.Opt, 0, len(names))
	for _, n := range names {
		out = append(out, views.Opt{Name: n, Swatch: swatches[strings.ToLower(n)]})
	}
	return out
}

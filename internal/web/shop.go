package web

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/storage"
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
	sh := views.Shop{Name: s.Name, Current: current, Whatsapp: strings.TrimPrefix(s.Whatsapp, "+")}
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
			Photo: a.photo(p.ImageKey, p.ImageWidths), Color: p.ImageColor, Sizes: p.Sizes,
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
	renderStatus(w, r, http.StatusOK, views.ShopHome(sh, a.cards(ps, 0)))
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
	sh, _, err := a.shop(r, p.CategoryID.Int64)
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
		Card:        views.Card{ID: p.ID, Name: p.Name, Price: now, Was: was, SoldOut: p.SoldOut, New: p.IsNew},
		Description: p.Description,
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
	renderStatus(w, r, http.StatusOK, views.ShopProduct(sh, pp))
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
	"beige": "#e7d8c0", "crema": "#f5efe0", "arena": "#d6c3a1", "dorado": "#d4a017", "plateado": "#c0c0c0",
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
	Name    string          `json:"n"`
	Price   int32           `json:"p"`
	URL     string          `json:"u"`
	SoldOut bool            `json:"x"`
	Combos  map[string]bool `json:"v"` // "talla|color" -> agotada
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
		out[id] = &cartProduct{Name: p.Name, Price: now, URL: "/p/" + id, SoldOut: p.SoldOut, Combos: map[string]bool{}}
	}
	for _, v := range vs {
		if p := out[strconv.FormatInt(v.ProductID, 10)]; p != nil {
			p.Combos[variantKey{v.Size, v.Color}.String()] = v.SoldOut
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

func (a *app) shopURL(r *http.Request, slug string) string {
	return strings.TrimSuffix(a.resetLink(r.Host, slug, ""), "/panel/recuperar?t=") + "/"
}

func (a *app) myStore(w http.ResponseWriter, r *http.Request) {
	s, err := a.q.GetStoreBySlug(r.Context(), StoreFrom(r.Context()).Slug) // sin caché: muestra lo recién guardado
	if err != nil {
		a.fail(w, r, err)
		return
	}
	renderStatus(w, r, http.StatusOK, views.MyStore(s.Name, a.shopURL(r, s.Slug), s.Whatsapp, "", r.URL.Query().Has("guardado")))
}

func (a *app) myStoreSave(w http.ResponseWriter, r *http.Request) {
	s := StoreFrom(r.Context())
	raw := r.PostFormValue("whatsapp")
	wa, ok := normalizeWhatsapp(raw)
	if !ok || len(raw) > 40 {
		renderStatus(w, r, http.StatusUnprocessableEntity, views.MyStore(s.Name, a.shopURL(r, s.Slug), raw,
			"Revisa el número: deben ser 10 dígitos si es de México, o empezar con + y la lada si es de otro país.", false))
		return
	}
	if err := a.q.UpdateStoreWhatsapp(r.Context(), db.UpdateStoreWhatsappParams{ID: s.ID, Whatsapp: wa}); err != nil {
		a.fail(w, r, err)
		return
	}
	a.tenants.forget(s.Slug)
	back(w, r, "/panel/tienda?guardado")
}

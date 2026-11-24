package web

import (
	"cmp"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/web/views"
)

// pathID lee {id}; un id inválido se trata igual que uno ajeno: 404.
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return 0, false
	}
	return id, true
}

func (a *app) fail(w http.ResponseWriter, r *http.Request, err error) {
	Log(r.Context()).Error("panel", "err", err)
	http.Error(w, "Error interno", http.StatusInternalServerError)
}

// notMine responde 404 (no 403) cuando el registro no es de esta tienda, para no confirmar que existe.
func notMine(w http.ResponseWriter, r *http.Request, rows int64) bool {
	if rows == 0 {
		http.NotFound(w, r)
		return true
	}
	return false
}

func back(w http.ResponseWriter, r *http.Request, to string) {
	http.Redirect(w, r, to, http.StatusSeeOther)
}

// ---- Categorías ----

func (a *app) categories(w http.ResponseWriter, r *http.Request) {
	a.renderCategories(w, r, http.StatusOK, "")
}

func (a *app) renderCategories(w http.ResponseWriter, r *http.Request, status int, errMsg string) {
	s := StoreFrom(r.Context())
	cats, err := a.q.ListCategories(r.Context(), s.ID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	renderStatus(w, r, status, views.Categories(s.Name, cats, errMsg))
}

func categoryName(r *http.Request) (string, bool) {
	name := strings.TrimSpace(r.PostFormValue("name"))
	return name, name != "" && utf8.RuneCountInString(name) <= 60
}

func (a *app) categoryCreate(w http.ResponseWriter, r *http.Request) {
	name, ok := categoryName(r)
	if !ok {
		a.renderCategories(w, r, http.StatusUnprocessableEntity, "El nombre debe tener de 1 a 60 caracteres.")
		return
	}
	if _, err := a.q.CreateCategory(r.Context(), db.CreateCategoryParams{StoreID: StoreFrom(r.Context()).ID, Name: name}); err != nil {
		a.fail(w, r, err)
		return
	}
	back(w, r, "/panel/categorias")
}

func (a *app) categoryRename(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	name, ok := categoryName(r)
	if !ok {
		a.renderCategories(w, r, http.StatusUnprocessableEntity, "El nombre debe tener de 1 a 60 caracteres.")
		return
	}
	n, err := a.q.RenameCategory(r.Context(), db.RenameCategoryParams{StoreID: StoreFrom(r.Context()).ID, ID: id, Name: name})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if notMine(w, r, n) {
		return
	}
	back(w, r, "/panel/categorias")
}

func (a *app) categoryDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	n, err := a.q.DeleteCategory(r.Context(), db.DeleteCategoryParams{StoreID: StoreFrom(r.Context()).ID, ID: id})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if notMine(w, r, n) {
		return
	}
	back(w, r, "/panel/categorias")
}

// categoryMove sube o baja una categoría un lugar. Es la alternativa sin arrastre (WCAG 2.5.7) y funciona sin JS.
func (a *app) categoryMove(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	s := StoreFrom(r.Context())
	cats, err := a.q.ListCategories(r.Context(), s.ID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	ids := make([]int64, len(cats))
	for i, c := range cats {
		ids[i] = c.ID
	}
	i := slices.Index(ids, id)
	if i < 0 {
		http.NotFound(w, r)
		return
	}
	j := i - 1
	if r.PostFormValue("dir") == "down" {
		j = i + 1
	}
	if j >= 0 && j < len(ids) {
		ids[i], ids[j] = ids[j], ids[i]
		if err := a.q.ReorderCategories(r.Context(), db.ReorderCategoriesParams{StoreID: s.ID, Ids: ids}); err != nil {
			a.fail(w, r, err)
			return
		}
	}
	back(w, r, "/panel/categorias")
}

// ---- Productos ----

func (a *app) products(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := StoreFrom(ctx)
	ps, err := a.q.ListPanelProducts(ctx, s.ID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	vs, err := a.q.ListAllVariants(ctx, s.ID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	cats, err := a.q.ListCategories(ctx, s.ID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	rows := make([]views.ProductRow, 0, len(ps))
	at := make(map[int64]int, len(ps))
	for i, p := range ps {
		at[p.ID] = i
		row := views.ProductRow{ID: p.ID, Name: p.Name, Category: p.Category, Thumb: a.photo(p.ImageKey, p.ImageWidths).Src,
			Price: views.Money(p.PriceCents), Active: p.Active, SoldOut: p.SoldOut, IsNew: p.IsNew, Unit: units[p.Unit]}
		if p.SalePriceCents.Valid && p.OnSale {
			row.Was, row.Price = row.Price, views.Money(p.SalePriceCents.Int32)
		}
		if p.Stock >= 0 {
			row.Stock, row.Low = formatQty(p.Stock), p.Stock <= p.Step*3
			row.SoldOut = row.SoldOut || p.Stock < p.Step
		}
		rows = append(rows, row)
	}
	for _, v := range vs {
		if i, ok := at[v.ProductID]; ok && v.Stock.Valid {
			rows[i].Combos = append(rows[i].Combos, views.StockRow{Key: variantKey{v.Size, v.Color}.String(), Label: variantLabel(v.Size, v.Color), Stock: formatQty(v.Stock.Int32)})
		}
	}
	renderStatus(w, r, http.StatusOK, views.Products(s.Name, rows, cats))
}

// productStock: ajuste rápido de existencias desde la lista (stock o stock-<talla|color>).
func (a *app) productStock(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "formulario mal formado", http.StatusBadRequest)
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	sid := StoreFrom(ctx).ID
	vs, err := a.q.ListVariants(ctx, db.ListVariantsParams{StoreID: sid, ProductID: id})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	err = a.tx(ctx, func(q db.Querier) error {
		if len(vs) == 0 {
			n, ok, err := parseQty(r.PostFormValue("stock"))
			if err != nil || !ok {
				return errBadStock
			}
			rows, err := q.SetProductStock(ctx, db.SetProductStockParams{StoreID: sid, ID: id, Stock: pgtype.Int4{Int32: n, Valid: true}})
			if err == nil && rows == 0 {
				return errNotMine
			}
			return err
		}
		for _, v := range vs {
			raw, sent := r.PostForm["stock-"+variantKey{v.Size, v.Color}.String()]
			if !sent {
				continue
			}
			n, ok, err := parseQty(raw[0])
			if err != nil || !ok {
				return errBadStock
			}
			if err := q.SetVariantStock(ctx, db.SetVariantStockParams{Stock: pgtype.Int4{Int32: n, Valid: true}, StoreID: sid, ProductID: id, Size: v.Size, Color: v.Color}); err != nil {
				return err
			}
		}
		return nil
	})
	switch {
	case errors.Is(err, errNotMine):
		http.NotFound(w, r)
	case errors.Is(err, errBadStock):
		http.Error(w, "La existencia debe ser un número como 10 o 2.5.", http.StatusUnprocessableEntity)
	case err != nil:
		a.fail(w, r, err)
	default:
		back(w, r, "/panel/productos?existencia="+strconv.FormatInt(id, 10))
	}
}

var errBadStock = errors.New("existencia no válida")

var toggleFlags = map[string]bool{"active": true, "sold_out": true, "is_new": true, "on_sale": true}

func (a *app) productToggle(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	flag := r.PathValue("flag")
	if !toggleFlags[flag] {
		http.NotFound(w, r)
		return
	}
	n, err := a.q.ToggleProductFlag(r.Context(), db.ToggleProductFlagParams{Flag: flag, StoreID: StoreFrom(r.Context()).ID, ID: id})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if notMine(w, r, n) {
		return
	}
	back(w, r, "/panel/#p"+strconv.FormatInt(id, 10))
}

func (a *app) productDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	sid := StoreFrom(r.Context()).ID
	imgs, err := a.q.ListProductImages(r.Context(), db.ListProductImagesParams{StoreID: sid, ProductID: id})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	n, err := a.q.DeleteProduct(r.Context(), db.DeleteProductParams{StoreID: sid, ID: id})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if notMine(w, r, n) {
		return
	}
	for _, im := range imgs { // las filas se borran en cascada; los archivos, aquí
		a.cleanup(r.Context(), photoKeys(im.R2Key, im.Widths))
	}
	back(w, r, "/panel/productos")
}

func (a *app) productNew(w http.ResponseWriter, r *http.Request) {
	a.renderProductForm(w, r, http.StatusOK, views.ProductForm{Unit: "pieza", Step: "1", Track: true, Saved: r.URL.Query().Has("guardado")})
}

func (a *app) productEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	s := StoreFrom(r.Context())
	p, err := a.q.GetProduct(r.Context(), db.GetProductParams{StoreID: s.ID, ID: id})
	if err != nil {
		a.notFoundOr500(w, r, err)
		return
	}
	vs, err := a.q.ListVariants(r.Context(), db.ListVariantsParams{StoreID: s.ID, ProductID: id})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	f := views.ProductForm{ID: p.ID, Name: p.Name, Description: p.Description, Price: formatPrice(p.PriceCents), Saved: r.URL.Query().Has("guardado"),
		Unit: p.Unit, Step: formatQty(p.Step), Track: p.Stock.Valid, OnSale: p.OnSale, IsNew: p.IsNew, SoldOut: p.SoldOut}
	if p.Stock.Valid {
		f.Stock = formatQty(p.Stock.Int32)
	}
	if p.SalePriceCents.Valid {
		f.SalePrice = formatPrice(p.SalePriceCents.Int32)
	}
	if p.CategoryID.Valid {
		f.CategoryID = p.CategoryID.Int64
	}
	var sizes, colors []string
	for _, v := range vs {
		if v.Size != "" && !slices.Contains(sizes, v.Size) {
			sizes = append(sizes, v.Size)
		}
		if v.Color != "" && !slices.Contains(colors, v.Color) {
			colors = append(colors, v.Color)
		}
		row := views.VariantRow{Key: variantKey{v.Size, v.Color}.String(), Label: variantLabel(v.Size, v.Color), SoldOut: v.SoldOut}
		if v.Stock.Valid {
			row.Stock, f.Track = formatQty(v.Stock.Int32), true
		}
		f.Variants = append(f.Variants, row)
	}
	f.Sizes, f.Colors = strings.Join(sizes, ", "), strings.Join(colors, ", ")
	f.PhotoOK = r.URL.Query().Get("foto") == "ok"
	f.PhotoError = photoErrors[r.URL.Query().Get("foto")]
	a.renderProductForm(w, r, http.StatusOK, f)
}

func variantLabel(size, color string) string {
	return strings.TrimSpace(strings.Join([]string{size, color}, " "))
}

func (a *app) renderProductForm(w http.ResponseWriter, r *http.Request, status int, f views.ProductForm) {
	s := StoreFrom(r.Context())
	cats, err := a.q.ListCategories(r.Context(), s.ID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if f.ID != 0 {
		imgs, err := a.q.ListProductImages(r.Context(), db.ListProductImagesParams{StoreID: s.ID, ProductID: f.ID})
		if err != nil {
			a.fail(w, r, err)
			return
		}
		f.Photos = a.photoViews(imgs)
	}
	renderStatus(w, r, status, views.ProductEdit(s.Name, cats, f))
}

func (a *app) notFoundOr500(w http.ResponseWriter, r *http.Request, err error) {
	if isNoRows(err) {
		http.NotFound(w, r)
		return
	}
	a.fail(w, r, err)
}

// productSave crea (sin {id}) o actualiza un producto con sus variantes en una sola transacción.
func (a *app) productSave(w http.ResponseWriter, r *http.Request) {
	var id int64
	if r.PathValue("id") != "" {
		var ok bool
		if id, ok = pathID(w, r); !ok {
			return
		}
	}
	f := views.ProductForm{
		ID:          id,
		Name:        strings.TrimSpace(r.PostFormValue("name")),
		Description: strings.TrimSpace(strings.ReplaceAll(r.PostFormValue("description"), "\r\n", "\n")),
		Price:       r.PostFormValue("price"),
		SalePrice:   r.PostFormValue("sale_price"),
		Sizes:       r.PostFormValue("sizes"),
		Colors:      r.PostFormValue("colors"),
		Unit:        cmp.Or(r.PostFormValue("unit"), "pieza"),
		Step:        r.PostFormValue("step"),
		Stock:       r.PostFormValue("stock"),
		Track:       r.PostFormValue("track") != "",
		IsNew:       r.PostFormValue("is_new") != "",
		SoldOut:     r.PostFormValue("sold_out") != "",
	}
	f.CategoryID, _ = strconv.ParseInt(r.PostFormValue("category_id"), 10, 64)

	price, err := parsePrice(f.Price)
	var sale pgtype.Int4
	step, _, stepErr := parseQty(cmp.Or(strings.TrimSpace(f.Step), "1"))
	switch {
	case f.Name == "" || utf8.RuneCountInString(f.Name) > 120:
		f.Error, f.Field = "El nombre debe tener de 1 a 120 caracteres.", "name"
	case utf8.RuneCountInString(f.Description) > 4000:
		f.Error, f.Field = "La descripción no puede pasar de 4000 caracteres.", "description"
	case err != nil:
		f.Error, f.Field = "Precio: "+err.Error()+".", "price"
	case units[f.Unit] == "":
		f.Error, f.Field = "Elige cómo se vende.", "unit"
	case stepErr != nil || step <= 0:
		f.Error, f.Field = "De cuánto en cuánto: debe ser un número mayor que 0, como 1 o 0.25.", "step"
	case f.Unit == "pieza" && step%1000 != 0:
		f.Error, f.Field = "Las piezas se venden enteras: de 1 en 1, de 2 en 2…", "step"
	}
	if f.Error == "" && strings.TrimSpace(f.SalePrice) != "" {
		sp, err := parsePrice(f.SalePrice)
		switch {
		case err != nil:
			f.Error, f.Field = "Precio de oferta: "+err.Error()+".", "sale_price"
		case sp >= price:
			f.Error, f.Field = "El precio de oferta debe ser menor que el precio normal.", "sale_price"
		default:
			sale = pgtype.Int4{Int32: sp, Valid: true}
		}
	}
	matrix := variantMatrix(splitList(f.Sizes, 20), splitList(f.Colors, 30))
	if f.Error == "" && len(matrix) > 100 {
		f.Error, f.Field = "Son demasiadas combinaciones de talla y color (máximo 100).", "sizes"
	}
	// Existencias: con inventario, una por producto o por combinación (vacío = 0).
	var stock pgtype.Int4
	stocks := map[string]pgtype.Int4{}
	if f.Error == "" && f.Track {
		check := func(raw string) pgtype.Int4 {
			n, _, err := parseQty(raw)
			if err != nil && f.Error == "" {
				f.Error, f.Field = "Existencia: "+err.Error()+".", "stock"
			}
			return pgtype.Int4{Int32: n, Valid: true}
		}
		if len(matrix) == 0 {
			stock = check(f.Stock)
		}
		for _, k := range matrix {
			stocks[k.String()] = check(r.PostFormValue("stock-" + k.String()))
		}
	}
	// Si vuelve con error, la cuadrícula conserva lo que escribió.
	for _, k := range matrix {
		f.Variants = append(f.Variants, views.VariantRow{Key: k.String(), Label: variantLabel(k.Size, k.Color),
			SoldOut: slices.Contains(r.PostForm["soldout"], k.String()), Stock: r.PostFormValue("stock-" + k.String())})
	}
	f.OnSale = sale.Valid
	if f.Error != "" {
		a.renderProductForm(w, r, http.StatusUnprocessableEntity, f)
		return
	}

	soldOut := map[string]bool{}
	for _, k := range r.PostForm["soldout"] {
		soldOut[k] = true
	}
	s := StoreFrom(r.Context())
	cat := pgtype.Int8{Int64: f.CategoryID, Valid: f.CategoryID > 0}
	err = a.tx(r.Context(), func(q db.Querier) error {
		if id == 0 {
			newID, err := q.CreateProduct(r.Context(), db.CreateProductParams{StoreID: s.ID, CategoryID: cat, Name: f.Name, Description: f.Description, PriceCents: price, SalePriceCents: sale,
				IsNew: f.IsNew, OnSale: f.OnSale, SoldOut: f.SoldOut && !f.Track, Unit: f.Unit, Step: step, Stock: stock})
			id = newID
			if err != nil {
				return err
			}
		} else {
			n, err := q.UpdateProduct(r.Context(), db.UpdateProductParams{StoreID: s.ID, ID: id, CategoryID: cat, Name: f.Name, Description: f.Description, PriceCents: price, SalePriceCents: sale,
				IsNew: f.IsNew, OnSale: f.OnSale, SoldOut: f.SoldOut && !f.Track, Unit: f.Unit, Step: step, Stock: stock})
			if err != nil {
				return err
			}
			if n == 0 {
				return errNotMine
			}
			if err := q.DeleteVariants(r.Context(), db.DeleteVariantsParams{StoreID: s.ID, ProductID: id}); err != nil {
				return err
			}
		}
		for _, k := range matrix {
			if err := q.CreateVariant(r.Context(), db.CreateVariantParams{StoreID: s.ID, ProductID: id, Size: k.Size, Color: k.Color,
				SoldOut: soldOut[k.String()] && !f.Track, Stock: stocks[k.String()]}); err != nil {
				return err
			}
		}
		return nil
	})
	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, errNotMine):
		http.NotFound(w, r)
	case errors.As(err, &pgErr) && pgErr.Code == "23503": // FK: la categoría no es de esta tienda
		f.Error, f.Field = "Esa categoría ya no existe.", "category_id"
		a.renderProductForm(w, r, http.StatusUnprocessableEntity, f)
	case err != nil:
		a.fail(w, r, err)
	case r.PostFormValue("then") == "nuevo": // "Guardar y agregar otro"
		back(w, r, "/panel/productos/nuevo?guardado=1")
	default:
		back(w, r, "/panel/productos/"+strconv.FormatInt(id, 10)+"?guardado=1")
	}
}

var errNotMine = errors.New("no es de esta tienda")

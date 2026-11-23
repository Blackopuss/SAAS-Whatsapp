package web

import (
	"cmp"
	"encoding/csv"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/web/views"
)

// Ciudad de México no tiene horario de verano desde 2022.
var mxTime = time.FixedZone("CST", -6*3600)

var salesTabs = map[string]int16{"pendientes": orderPending, "vendidos": orderSold, "no-vendidos": orderLost}

func (a *app) sales(w http.ResponseWriter, r *http.Request) {
	a.renderSales(w, r, http.StatusOK, "", 0)
}

// renderSales pinta la lista de pedidos de un estado; errMsg/focus señalan el pedido que no se pudo vender.
func (a *app) renderSales(w http.ResponseWriter, r *http.Request, status int, errMsg string, focus int64) {
	ctx := r.Context()
	s := StoreFrom(ctx)
	tab := r.URL.Query().Get("estado")
	st, ok := salesTabs[tab]
	if !ok {
		tab, st = "pendientes", orderPending
	}
	os, err := a.q.ListOrders(ctx, db.ListOrdersParams{StoreID: s.ID, Status: st})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	ids := make([]int64, 0, len(os))
	for _, o := range os {
		ids = append(ids, o.ID)
	}
	items, err := a.q.ListOrderItems(ctx, db.ListOrderItemsParams{StoreID: s.ID, OrderIds: ids})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	byOrder := map[int64][]views.OrderItem{}
	for _, it := range items {
		v := views.OrderItem{ID: it.ID, Name: it.Name, Variant: variantLabel(it.Size, it.Color), Qty: formatQty(it.Qty), Unit: units[it.Unit],
			Price: views.Money(it.PriceCents), Line: views.Money(int32(lineTotal(it.PriceCents, it.Qty))), Step: formatQty(it.Step)} // #nosec G115 -- total acotado al crear el pedido
		if it.Stock >= 0 {
			v.Stock = formatQty(it.Stock)
			v.Short = it.Stock < it.Qty
		}
		byOrder[it.OrderID] = append(byOrder[it.OrderID], v)
	}
	out := make([]views.OrderView, 0, len(os))
	for _, o := range os {
		out = append(out, views.OrderView{ID: o.ID, Code: o.Code, Created: o.CreatedAt.Time.In(mxTime).Format("02/01/2006 15:04"),
			Total: views.Money(o.TotalCents), Reason: lostReasons[o.Reason], Items: byOrder[o.ID]})
	}
	renderStatus(w, r, status, views.Sales(s.Name, tab, out, errMsg, focus))
}

// orderAction carga el pedido (bloqueado) y sus renglones dentro de una transacción.
func (a *app) orderAction(w http.ResponseWriter, r *http.Request, fn func(q db.Querier, o db.Order, items []db.ListOrderItemsRow) error) error {
	id, ok := pathID(w, r)
	if !ok {
		return errHandled
	}
	ctx := r.Context()
	sid := StoreFrom(ctx).ID
	return a.tx(ctx, func(q db.Querier) error {
		o, err := q.GetOrderForUpdate(ctx, db.GetOrderForUpdateParams{StoreID: sid, ID: id})
		if isNoRows(err) {
			return errNotMine
		}
		if err != nil {
			return err
		}
		items, err := q.ListOrderItems(ctx, db.ListOrderItemsParams{StoreID: sid, OrderIds: []int64{id}})
		if err != nil {
			return err
		}
		return fn(q, o, items)
	})
}

var (
	errHandled  = errors.New("ya respondido")
	errDecided  = errors.New("el pedido ya no está pendiente")
	errNotFound = errors.New("no encontrado")
	errEmpty    = errors.New("pedido sin renglones")
)

// finish responde según el error de una acción sobre un pedido y regresa a la pestaña indicada.
func (a *app) finish(w http.ResponseWriter, r *http.Request, err error, tab string) {
	switch {
	case errors.Is(err, errHandled):
	case errors.Is(err, errNotMine), errors.Is(err, errNotFound):
		http.NotFound(w, r)
	case errors.Is(err, errDecided):
		back(w, r, "/panel/ventas?estado="+tab)
	case err != nil:
		a.fail(w, r, err)
	default:
		back(w, r, "/panel/ventas?estado="+tab)
	}
}

func (a *app) orderSell(w http.ResponseWriter, r *http.Request) {
	var short db.ListOrderItemsRow
	sid := StoreFrom(r.Context()).ID
	err := a.orderAction(w, r, func(q db.Querier, o db.Order, items []db.ListOrderItemsRow) error {
		if o.Status != orderPending {
			return errDecided
		}
		if len(items) == 0 {
			return errEmpty
		}
		var err error
		if short, err = takeStock(r, q, sid, items); err != nil {
			return err
		}
		return q.SetOrderStatus(r.Context(), db.SetOrderStatusParams{Status: orderSold, StoreID: sid, ID: o.ID})
	})
	if errors.Is(err, errEmpty) {
		id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
		a.renderSales(w, r, http.StatusConflict, "El pedido ya no tiene productos. Márcalo como no vendido.", id)
		return
	}
	if errors.Is(err, errNoStock) {
		msg := "No alcanza la existencia de " + strings.TrimSpace(short.Name+" "+variantLabel(short.Size, short.Color))
		if short.Stock >= 0 {
			msg += " (quedan " + formatQty(short.Stock) + " " + units[short.Unit] + ")"
		}
		id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
		a.renderSales(w, r, http.StatusConflict, msg+". Ajusta el pedido o la existencia del producto.", id)
		return
	}
	a.finish(w, r, err, "vendidos")
}

func (a *app) orderLose(w http.ResponseWriter, r *http.Request) {
	reason, _ := strconv.ParseInt(r.PostFormValue("reason"), 10, 16)
	if reason < 1 || reason > 4 { // el 5 solo lo pone el proceso automático
		http.Error(w, "elige un motivo", http.StatusUnprocessableEntity)
		return
	}
	sid := StoreFrom(r.Context()).ID
	err := a.orderAction(w, r, func(q db.Querier, o db.Order, _ []db.ListOrderItemsRow) error {
		if o.Status != orderPending {
			return errDecided
		}
		return q.SetOrderStatus(r.Context(), db.SetOrderStatusParams{Status: orderLost, Reason: int16(reason), StoreID: sid, ID: o.ID}) // #nosec G115 -- reason validado en 1..4 arriba
	})
	a.finish(w, r, err, "no-vendidos")
}

// orderUndo regresa un pedido a pendiente; si estaba vendido, devuelve su inventario.
func (a *app) orderUndo(w http.ResponseWriter, r *http.Request) {
	sid := StoreFrom(r.Context()).ID
	err := a.orderAction(w, r, func(q db.Querier, o db.Order, items []db.ListOrderItemsRow) error {
		if o.Status == orderPending {
			return errDecided
		}
		if o.Status == orderSold {
			if err := returnStock(r, q, sid, items); err != nil {
				return err
			}
		}
		return q.SetOrderStatus(r.Context(), db.SetOrderStatusParams{Status: orderPending, StoreID: sid, ID: o.ID})
	})
	a.finish(w, r, err, "pendientes")
}

// orderAdjust cambia cantidades de un pedido pendiente (qty-<renglón>); 0 quita el renglón.
func (a *app) orderAdjust(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "formulario mal formado", http.StatusBadRequest)
		return
	}
	sid := StoreFrom(r.Context()).ID
	var bad string
	err := a.orderAction(w, r, func(q db.Querier, o db.Order, items []db.ListOrderItemsRow) error {
		if o.Status != orderPending {
			return errDecided
		}
		ctx := r.Context()
		for _, it := range items {
			raw, sent := r.PostForm["qty-"+strconv.FormatInt(it.ID, 10)]
			if !sent {
				continue
			}
			qty, ok, err := parseQty(raw[0])
			switch {
			case err != nil || !ok || qty%it.Step != 0:
				bad = it.Name + ": la cantidad debe ir de " + formatQty(it.Step) + " en " + formatQty(it.Step) + "."
				return errHandled
			case qty == 0:
				if _, err := q.DeleteOrderItem(ctx, db.DeleteOrderItemParams{StoreID: sid, OrderID: o.ID, ID: it.ID}); err != nil {
					return err
				}
			case qty != it.Qty:
				if _, err := q.SetOrderItemQty(ctx, db.SetOrderItemQtyParams{Qty: qty, StoreID: sid, OrderID: o.ID, ID: it.ID}); err != nil {
					return err
				}
			}
		}
		return q.SetOrderTotal(ctx, db.SetOrderTotalParams{StoreID: sid, OrderID: o.ID})
	})
	if bad != "" {
		id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
		a.renderSales(w, r, http.StatusUnprocessableEntity, bad, id)
		return
	}
	a.finish(w, r, err, "pendientes")
}

// ---- Resumen ----

type period struct {
	key, label  string
	since, till time.Time
	bucket      string // "day" o "month"
}

var periods = []struct{ key, label string }{{"mes", "Este mes"}, {"mes-pasado", "Mes pasado"}, {"3-meses", "Últimos 3 meses"}, {"ano", "Este año"}}

func periodFor(key string, now time.Time) period {
	now = now.In(mxTime)
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, mxTime)
	switch key {
	case "mes-pasado":
		return period{key, "Mes pasado", month.AddDate(0, -1, 0), month, "day"}
	case "3-meses":
		return period{key, "Últimos 3 meses", month.AddDate(0, -2, 0), month.AddDate(0, 1, 0), "month"}
	case "ano":
		return period{key, "Este año", time.Date(now.Year(), 1, 1, 0, 0, 0, 0, mxTime), time.Date(now.Year()+1, 1, 1, 0, 0, 0, 0, mxTime), "month"}
	}
	return period{"mes", "Este mes", month, month.AddDate(0, 1, 0), "day"}
}

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func (a *app) salesSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := StoreFrom(ctx)
	p := periodFor(r.URL.Query().Get("periodo"), time.Now())
	totals, err := a.q.SalesTotals(ctx, db.SalesTotalsParams{StoreID: s.ID, Since: ts(p.since), Until: ts(p.till)})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	series, err := a.q.SalesSeries(ctx, db.SalesSeriesParams{Bucket: p.bucket, StoreID: s.ID, Since: ts(p.since), Until: ts(p.till)})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	items, err := a.q.SalesItems(ctx, db.SalesItemsParams{StoreID: s.ID, Since: ts(p.since), Until: ts(p.till)})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	low, err := a.q.LowStock(ctx, s.ID)
	if err != nil {
		a.fail(w, r, err)
		return
	}

	v := views.SalesSummary{Period: p.key}
	for _, x := range periods {
		v.Periods = append(v.Periods, views.Choice{Value: x.key, Label: x.label})
	}
	var soldCents, lostCents int64
	reasons := map[int16]*views.ReasonRow{}
	for _, t := range totals {
		switch t.Status {
		case orderPending:
			v.Pending += t.Orders
		case orderSold:
			v.Sold += t.Orders
			soldCents += t.Cents
		case orderLost:
			v.Lost += t.Orders
			lostCents += t.Cents
			if reasons[t.Reason] == nil {
				reasons[t.Reason] = &views.ReasonRow{Label: lostReasons[t.Reason]}
			}
			reasons[t.Reason].Orders += t.Orders
			reasons[t.Reason].Cents += t.Cents
		}
	}
	v.Revenue, v.LostCents = soldCents, lostCents
	if v.Sold > 0 {
		v.Ticket = soldCents / int64(v.Sold)
	}
	if decided := v.Sold + v.Lost; decided > 0 {
		v.Conversion = int32((int64(v.Sold)*100 + int64(decided)/2) / int64(decided)) // #nosec G115 -- 0..100
	}
	for _, rr := range reasons {
		v.Reasons = append(v.Reasons, *rr)
	}
	slices.SortFunc(v.Reasons, func(a, b views.ReasonRow) int { return cmp.Compare(b.Orders, a.Orders) })

	// Serie completa: los días o meses sin ventas también salen (en 0) para que la gráfica no engañe.
	got := map[string]db.SalesSeriesRow{}
	for _, x := range series {
		got[x.Day.Time.Format("2006-01-02")] = x
	}
	for d := p.since; d.Before(p.till); {
		k := d.Format("2006-01-02")
		label := strconv.Itoa(d.Day())
		next := d.AddDate(0, 0, 1)
		if p.bucket == "month" {
			label, next = monthsShort[d.Month()-1], d.AddDate(0, 1, 0)
		}
		v.Series = append(v.Series, views.SeriesPoint{Label: label, Sold: got[k].Sold, Lost: got[k].Lost})
		d = next
	}

	cats := map[string]int64{}
	for _, it := range items {
		row := views.ItemRow{Name: it.Name, Variant: variantLabel(it.Size, it.Color), Qty: formatQty(int32(min(it.Qty, maxQty))), Unit: units[it.Unit], Cents: it.Cents} // #nosec G115 -- acotado
		if it.Status == orderSold {
			v.Top = append(v.Top, row)
			cats[cmp.Or(it.Category, "Sin categoría")] += it.Cents
		} else {
			v.TopLost = append(v.TopLost, row)
		}
	}
	byCents := func(a, b views.ItemRow) int { return cmp.Compare(b.Cents, a.Cents) }
	slices.SortFunc(v.Top, byCents)
	slices.SortFunc(v.TopLost, byCents)
	v.Top, v.TopLost = v.Top[:min(10, len(v.Top))], v.TopLost[:min(10, len(v.TopLost))]
	for name, c := range cats {
		v.Categories = append(v.Categories, views.ItemRow{Name: name, Cents: c})
	}
	slices.SortFunc(v.Categories, byCents)
	for _, l := range low {
		v.LowStock = append(v.LowStock, views.ItemRow{ID: l.ID, Name: l.Name, Variant: variantLabel(l.Size, l.Color), Qty: formatQty(l.Stock), Unit: units[l.Unit]})
	}
	renderStatus(w, r, http.StatusOK, views.SalesSummaryPage(s.Name, v))
}

var monthsShort = []string{"ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic"}

var statusNames = map[int16]string{orderPending: "pendiente", orderSold: "vendido", orderLost: "no vendido"}

// salesCSV: renglones del periodo para que la vendedora lleve sus registros en Excel.
func (a *app) salesCSV(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := StoreFrom(ctx)
	p := periodFor(r.URL.Query().Get("periodo"), time.Now())
	rows, err := a.q.SalesCSV(ctx, db.SalesCSVParams{StoreID: s.ID, Since: ts(p.since), Until: ts(p.till)})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="ventas-`+p.key+`.csv"`)
	_, _ = w.Write([]byte("\ufeff")) // BOM: Excel abre bien los acentos
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"Pedido", "Fecha", "Estado", "Motivo", "Producto", "Categoría", "Talla", "Color", "Cantidad", "Unidad", "Precio", "Importe"})
	for _, x := range rows {
		_ = cw.Write([]string{x.Code, x.CreatedAt.Time.In(mxTime).Format("2006-01-02 15:04"), statusNames[x.Status], lostReasons[x.Reason],
			csvSafe(x.Name), csvSafe(x.Category), csvSafe(x.Size), csvSafe(x.Color), formatQty(x.Qty), units[x.Unit],
			formatPrice(x.PriceCents), formatPrice(int32(lineTotal(x.PriceCents, x.Qty)))}) // #nosec G115 -- acotado al crear el pedido
	}
	cw.Flush()
}

// csvSafe evita que Excel interprete como fórmula un texto que escribió la vendedora.
func csvSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

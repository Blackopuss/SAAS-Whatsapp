package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
)

// Estados y motivos de orders (ver migración 00009).
const (
	orderPending int16 = 0
	orderSold    int16 = 1
	orderLost    int16 = 2
)

var lostReasons = map[int16]string{1: "Sin existencia", 2: "La clienta no respondió", 3: "Precio", 4: "Otro", 5: "Sin respuesta en 7 días"}

var orderCode = regexp.MustCompile(`^[A-HJ-NP-Z2-9]{6}$`)

// available: cuánto se puede vender (milésimas); -1 = sin inventario y no está agotado.
func available(soldOut bool, stock pgtype.Int4) int32 {
	switch {
	case soldOut:
		return 0
	case stock.Valid:
		return stock.Int32
	default:
		return -1
	}
}

// lineTotal: precio por unidad × cantidad en milésimas, redondeado al centavo.
func lineTotal(price, qty int32) int64 { return (int64(price)*int64(qty) + 500) / 1000 }

// customerName limpia el nombre que escribe la clienta (sin espacios de sobra) y lo valida:
// 2 a 60 caracteres, solo letras, números, espacios y ' . -  (sin * _ ~ que WhatsApp toma como formato,
// ni = + @ que Excel toma como fórmula). Devuelve "" si no sirve.
var customerChars = regexp.MustCompile(`^[\p{L}\p{M}\p{N}][\p{L}\p{M}\p{N} '.\-]*$`)

const maxCustomer = 60

func customerName(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if n := utf8.RuneCountInString(s); n < 2 || n > maxCustomer || !customerChars.MatchString(s) {
		return ""
	}
	return s
}

type orderIn struct {
	Code  string `json:"code"`
	Name  string `json:"name"` // nombre para el pedido
	Items []struct {
		ID    int64  `json:"id"`
		Size  string `json:"size"`
		Color string `json:"color"`
		Qty   int32  `json:"qty"` // milésimas
	} `json:"items"`
}

// orderOut: el pedido tal como quedó registrado; con esto arma el celular el mensaje de WhatsApp,
// así el nombre y los importes del mensaje son los mismos que ve la vendedora en Ventas.
type orderOut struct {
	Code  string         `json:"code"`
	Name  string         `json:"name"`
	Total int64          `json:"total"` // centavos
	Items []orderOutItem `json:"items"`
}

type orderOutItem struct {
	Name     string `json:"name"`
	Size     string `json:"size"`
	Color    string `json:"color"`
	Qty      int32  `json:"qty"`  // milésimas
	Unit     string `json:"unit"` // pza, kg…
	Subtotal int64  `json:"subtotal"`
}

// shopOrder: la clienta tocó "Preparar pedido" en el carrito. Precio, nombre del producto y existencia salen de la base;
// del celular solo se toma su nombre y qué producto, talla, color y cuánto. Queda pendiente: no aparta inventario
// ni se marca vendido; la vendedora lo confirma en Ventas.
func (a *app) shopOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := StoreFrom(ctx)
	if !a.orderLimit.allow(strconv.FormatInt(s.ID, 10) + "|" + limitIP(r)) {
		Log(ctx).Warn("pedido frenado por límite", "event", "order_limited", "ip", clientIP(r))
		w.Header().Set("Retry-After", "3600")
		http.Error(w, "Demasiados pedidos seguidos. Intenta más tarde.", http.StatusTooManyRequests)
		return
	}
	var in orderIn
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&in); err != nil || !orderCode.MatchString(in.Code) || len(in.Items) == 0 || len(in.Items) > 50 {
		http.Error(w, "pedido mal formado", http.StatusBadRequest)
		return
	}
	name := customerName(in.Name)
	if name == "" {
		http.Error(w, "Escribe tu nombre para el pedido (de 2 a 60 letras).", http.StatusUnprocessableEntity)
		return
	}
	ids := make([]int64, 0, len(in.Items))
	for _, it := range in.Items {
		ids = append(ids, it.ID)
	}
	ps, err := a.q.OrderProducts(ctx, db.OrderProductsParams{StoreID: s.ID, Ids: ids})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	vs, err := a.q.OrderVariants(ctx, db.OrderVariantsParams{StoreID: s.ID, Ids: ids})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	prods := make(map[int64]db.OrderProductsRow, len(ps))
	for _, p := range ps {
		prods[p.ID] = p
	}
	hasVariants := map[int64]bool{}
	combos := map[string]int32{}
	for _, v := range vs {
		hasVariants[v.ProductID] = true
		combos[strconv.FormatInt(v.ProductID, 10)+"|"+variantKey{v.Size, v.Color}.String()] = available(v.SoldOut, v.Stock)
	}

	items := make([]db.AddOrderItemParams, 0, len(in.Items))
	var total int64
	for _, it := range in.Items {
		p, ok := prods[it.ID]
		if !ok || it.Qty <= 0 || it.Qty > maxQty || it.Qty%p.Step != 0 {
			http.Error(w, "producto o cantidad no válidos", http.StatusUnprocessableEntity)
			return
		}
		avail := available(p.SoldOut, p.Stock)
		if hasVariants[it.ID] {
			k := strconv.FormatInt(it.ID, 10) + "|" + variantKey{it.Size, it.Color}.String()
			v, ok := combos[k]
			if !ok {
				http.Error(w, "esa combinación no existe", http.StatusUnprocessableEntity)
				return
			}
			if !p.SoldOut { // con variantes, la existencia es la de la combinación; agotado a mano manda
				avail = v
			}
		} else if it.Size != "" || it.Color != "" {
			http.Error(w, "esa combinación no existe", http.StatusUnprocessableEntity)
			return
		}
		if avail != -1 && it.Qty > avail {
			http.Error(w, "ya no hay suficiente de "+p.Name, http.StatusConflict)
			return
		}
		price, _ := salePrice(p.PriceCents, p.OnSale, p.SalePriceCents.Int32, p.SalePriceCents.Valid)
		total += lineTotal(price, it.Qty)
		items = append(items, db.AddOrderItemParams{StoreID: s.ID, ProductID: pgtype.Int8{Int64: p.ID, Valid: true}, Name: p.Name, Category: p.Category,
			Size: it.Size, Color: it.Color, Unit: p.Unit, Qty: it.Qty, PriceCents: price})
	}
	if total > maxPriceCents*10 {
		http.Error(w, "pedido demasiado grande", http.StatusUnprocessableEntity)
		return
	}
	var out orderOut
	err = a.tx(ctx, func(q db.Querier) error {
		id, err := q.CreateOrder(ctx, db.CreateOrderParams{StoreID: s.ID, Code: in.Code, TotalCents: int32(total), Customer: name}) // #nosec G115 -- acotado arriba
		if isNoRows(err) {
			// Reintento del mismo pedido: se responde lo que ya quedó guardado, sin duplicar.
			o, err := q.GetOrderByCode(ctx, db.GetOrderByCodeParams{StoreID: s.ID, Code: in.Code})
			if err != nil {
				return err
			}
			saved, err := q.ListOrderItems(ctx, db.ListOrderItemsParams{StoreID: s.ID, OrderIds: []int64{o.ID}})
			if err != nil {
				return err
			}
			out = orderOut{Code: o.Code, Name: o.Customer, Total: int64(o.TotalCents)}
			for _, it := range saved {
				out.Items = append(out.Items, orderOutItem{it.Name, it.Size, it.Color, it.Qty, units[it.Unit], lineTotal(it.PriceCents, it.Qty)})
			}
			return nil
		}
		if err != nil {
			return err
		}
		out = orderOut{Code: in.Code, Name: name, Total: total}
		for _, it := range items {
			it.OrderID = id
			if err := q.AddOrderItem(ctx, it); err != nil {
				return err
			}
			out.Items = append(out.Items, orderOutItem{it.Name, it.Size, it.Color, it.Qty, units[it.Unit], lineTotal(it.PriceCents, it.Qty)})
		}
		return nil
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(out)
}

var errNoStock = errors.New("sin existencia")

// takeStock descuenta lo de un pedido; si algo no alcanza devuelve errNoStock y el renglón, y la transacción se revierte.
func takeStock(r *http.Request, q db.Querier, storeID int64, items []db.ListOrderItemsRow) (db.ListOrderItemsRow, error) {
	ctx := r.Context()
	for _, it := range items {
		if !it.ProductID.Valid { // producto borrado: ya no hay inventario que mover
			continue
		}
		var n int64
		var err error
		if it.Size == "" && it.Color == "" {
			n, err = q.TakeProductStock(ctx, db.TakeProductStockParams{Qty: it.Qty, StoreID: storeID, ID: it.ProductID.Int64})
		} else {
			n, err = q.TakeVariantStock(ctx, db.TakeVariantStockParams{Qty: it.Qty, StoreID: storeID, ProductID: it.ProductID.Int64, Size: it.Size, Color: it.Color})
		}
		if err != nil {
			return it, err
		}
		if n == 0 {
			return it, errNoStock
		}
	}
	return db.ListOrderItemsRow{}, nil
}

func returnStock(r *http.Request, q db.Querier, storeID int64, items []db.ListOrderItemsRow) error {
	ctx := r.Context()
	for _, it := range items {
		if !it.ProductID.Valid {
			continue
		}
		var err error
		if it.Size == "" && it.Color == "" {
			err = q.ReturnProductStock(ctx, db.ReturnProductStockParams{Qty: it.Qty, StoreID: storeID, ID: it.ProductID.Int64})
		} else {
			err = q.ReturnVariantStock(ctx, db.ReturnVariantStockParams{Qty: it.Qty, StoreID: storeID, ProductID: it.ProductID.Int64, Size: it.Size, Color: it.Color})
		}
		if err != nil {
			return err
		}
	}
	return nil
}

package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"

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

type orderIn struct {
	Code  string `json:"code"`
	Items []struct {
		ID    int64  `json:"id"`
		Size  string `json:"size"`
		Color string `json:"color"`
		Qty   int32  `json:"qty"` // milésimas
	} `json:"items"`
}

// shopOrder: la clienta tocó "Enviar pedido por WhatsApp". Precio, nombre y existencia salen de la base;
// del celular solo se toma qué producto, talla, color y cuánto. No aparta inventario: se descuenta al vender.
func (a *app) shopOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := StoreFrom(ctx)
	if !a.orderLimit.allow(strconv.FormatInt(s.ID, 10) + "|" + limitIP(r)) {
		Log(ctx).Warn("pedido frenado por límite", "event", "order_limited", "ip", clientIP(r))
		http.Error(w, "Demasiados pedidos seguidos. Intenta más tarde.", http.StatusTooManyRequests)
		return
	}
	var in orderIn
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&in); err != nil || !orderCode.MatchString(in.Code) || len(in.Items) == 0 || len(in.Items) > 50 {
		http.Error(w, "pedido mal formado", http.StatusBadRequest)
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
	err = a.tx(ctx, func(q db.Querier) error {
		id, err := q.CreateOrder(ctx, db.CreateOrderParams{StoreID: s.ID, Code: in.Code, TotalCents: int32(total)}) // #nosec G115 -- acotado arriba
		if isNoRows(err) {
			return nil // reintento del mismo pedido
		}
		if err != nil {
			return err
		}
		for _, it := range items {
			it.OrderID = id
			if err := q.AddOrderItem(ctx, it); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

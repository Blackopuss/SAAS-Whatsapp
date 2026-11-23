package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/auth"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
)

// TestOrders: pedido desde la tienda con precio de la base, vender descuenta, deshacer devuelve,
// no se vende sin existencia, otra tienda recibe 404 y hay límite de pedidos por IP. Postgres real.
func TestOrders(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("sin TEST_DATABASE_URL")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	q := db.New(pool)

	suffix := strconv.FormatInt(time.Now().UnixNano()%1e9, 36)
	hash, _ := auth.Hash("contraseña-segura")
	mk := func(slug string) db.CreateStoreRow {
		s, err := q.CreateStore(ctx, db.CreateStoreParams{Slug: slug, Name: slug})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := q.CreateUser(ctx, db.CreateUserParams{StoreID: s.ID, Email: slug + "@x.mx", PasswordHash: hash}); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(ctx, "DELETE FROM stores WHERE id = $1", s.ID) })
		return s
	}
	a, b := mk("ord-a-"+suffix), mk("ord-b-"+suffix)

	// Churros: $120 el kg, de 0.25 en 0.25, hay 1 kg. Blusa: $300, talla M con 2 piezas.
	churros, err := q.CreateProduct(ctx, db.CreateProductParams{StoreID: a.ID, Name: "Churros", PriceCents: 12000, Unit: "kg", Step: 250, Stock: pgtype.Int4{Int32: 1000, Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	blusa, err := q.CreateProduct(ctx, db.CreateProductParams{StoreID: a.ID, Name: "Blusa", PriceCents: 30000, Unit: "pieza", Step: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.CreateVariant(ctx, db.CreateVariantParams{StoreID: a.ID, ProductID: blusa, Size: "M", Stock: pgtype.Int4{Int32: 2000, Valid: true}}); err != nil {
		t.Fatal(err)
	}

	h := Routes(Deps{BaseDomain: "tiendas.test", DB: pool, Q: q, MediaURL: "/media/",
		Tx: func(ctx context.Context, fn func(db.Querier) error) error { return db.InTx(ctx, pool, fn) }})
	aHost, bHost := "http://"+a.Slug+".tiendas.test", "http://"+b.Slug+".tiendas.test"
	order := func(host, ip, body string, headers ...string) int {
		r := httptest.NewRequest("POST", host+"/pedido", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		for i := 0; i+1 < len(headers); i += 2 {
			r.Header.Set(headers[i], headers[i+1])
		}
		r.RemoteAddr = ip + ":5555"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec.Code
	}
	item := func(id int64, size string, qty int) string {
		return `{"id":` + strconv.FormatInt(id, 10) + `,"size":"` + size + `","color":"","qty":` + strconv.Itoa(qty) + `}`
	}

	// Validaciones: cantidad fuera de paso, más de lo que hay, combinación inexistente, producto de otra tienda, código malo.
	for name, c := range map[string]struct {
		body string
		want int
	}{
		"fuera de paso":     {`{"code":"AAAAA2","items":[` + item(churros, "", 300) + `]}`, 422},
		"sin existencia":    {`{"code":"AAAAA3","items":[` + item(churros, "", 1250) + `]}`, 409},
		"talla inexistente": {`{"code":"AAAAA4","items":[` + item(blusa, "XL", 1000) + `]}`, 422},
		"código malo":       {`{"code":"aaa","items":[` + item(churros, "", 250) + `]}`, 400},
	} {
		if got := order(aHost, "198.51.100.1", c.body); got != c.want {
			t.Errorf("%s: status %d, quiero %d", name, got, c.want)
		}
	}
	if got := order(bHost, "198.51.100.1", `{"code":"AAAAA5","items":[`+item(churros, "", 250)+`]}`); got != 422 {
		t.Errorf("producto de A pedido en B: status %d, quiero 422", got)
	}
	if got := order(aHost, "198.51.100.1", `{"code":"AAAAA6","items":[`+item(churros, "", 250)+`]}`, "Sec-Fetch-Site", "cross-site"); got != http.StatusForbidden {
		t.Errorf("pedido desde otro sitio: status %d, quiero 403", got)
	}

	// Pedido válido: 0.75 kg de churros + 2 blusas M. El precio que mande el celular no importa.
	body := `{"code":"PEDXD2","items":[` + item(churros, "", 750) + `,` + item(blusa, "M", 2000) + `]}`
	if got := order(aHost, "198.51.100.2", body); got != http.StatusNoContent {
		t.Fatalf("pedido válido: status %d", got)
	}
	if got := order(aHost, "198.51.100.2", body); got != http.StatusNoContent { // reintento: no duplica
		t.Fatalf("reintento: status %d", got)
	}
	os, err := q.ListOrders(ctx, db.ListOrdersParams{StoreID: a.ID, Status: orderPending})
	if err != nil || len(os) != 1 {
		t.Fatalf("pendientes = %d, %v; quiero 1", len(os), err)
	}
	if want := int32(9000 + 60000); os[0].TotalCents != want {
		t.Errorf("total = %d, quiero %d", os[0].TotalCents, want)
	}
	oid := strconv.FormatInt(os[0].ID, 10)

	stockOf := func() (int32, int32) {
		p, _ := q.GetProduct(ctx, db.GetProductParams{StoreID: a.ID, ID: churros})
		vs, _ := q.ListVariants(ctx, db.ListVariantsParams{StoreID: a.ID, ProductID: blusa})
		return p.Stock.Int32, vs[0].Stock.Int32
	}

	bCookie := login(t, h, b.Slug+".tiendas.test", b.Slug+"@x.mx")
	for _, act := range []string{"vender", "no-vender", "deshacer", "ajustar"} {
		if rec := do(h, "POST", bHost+"/panel/ventas/"+oid+"/"+act, url.Values{"reason": {"1"}}, bCookie); rec.Code != http.StatusNotFound {
			t.Errorf("B intenta %s el pedido de A: status %d, quiero 404", act, rec.Code)
		}
	}

	aCookie := login(t, h, a.Slug+".tiendas.test", a.Slug+"@x.mx")
	if rec := do(h, "GET", aHost+"/panel/ventas", nil, aCookie); rec.Code != 200 || !strings.Contains(rec.Body.String(), "PEDXD2") {
		t.Errorf("lista de ventas: %d", rec.Code)
	}
	if rec := do(h, "POST", aHost+"/panel/ventas/"+oid+"/vender", url.Values{}, aCookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("vender: status %d %s", rec.Code, rec.Body.String())
	}
	if c, m := stockOf(); c != 250 || m != 0 {
		t.Errorf("tras vender: churros %d, blusa M %d; quiero 250 y 0", c, m)
	}
	if rec := do(h, "POST", aHost+"/panel/ventas/"+oid+"/deshacer", url.Values{}, aCookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("deshacer: status %d", rec.Code)
	}
	if c, m := stockOf(); c != 1000 || m != 2000 {
		t.Errorf("tras deshacer: churros %d, blusa M %d; quiero 1000 y 2000", c, m)
	}

	// Sin existencia suficiente no vende ni toca nada.
	if err := q.SetVariantStock(ctx, db.SetVariantStockParams{Stock: pgtype.Int4{Int32: 1000, Valid: true}, StoreID: a.ID, ProductID: blusa, Size: "M"}); err != nil {
		t.Fatal(err)
	}
	if rec := do(h, "POST", aHost+"/panel/ventas/"+oid+"/vender", url.Values{}, aCookie); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "No alcanza") {
		t.Errorf("vender sin existencia: status %d", rec.Code)
	}
	if c, m := stockOf(); c != 1000 || m != 1000 {
		t.Errorf("tras fallar: churros %d, blusa M %d; quiero 1000 y 1000", c, m)
	}

	// Ajustar a 1 blusa y vender; luego no vender con motivo ya no aplica (ya está vendido).
	items, _ := q.ListOrderItems(ctx, db.ListOrderItemsParams{StoreID: a.ID, OrderIds: []int64{os[0].ID}})
	adj := url.Values{}
	for _, it := range items {
		if it.Size == "M" {
			adj.Set("qty-"+strconv.FormatInt(it.ID, 10), "1")
		}
	}
	if rec := do(h, "POST", aHost+"/panel/ventas/"+oid+"/ajustar", adj, aCookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("ajustar: status %d", rec.Code)
	}
	if rec := do(h, "POST", aHost+"/panel/ventas/"+oid+"/vender", url.Values{}, aCookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("vender ajustado: status %d", rec.Code)
	}
	if o, _ := q.GetOrderForUpdate(ctx, db.GetOrderForUpdateParams{StoreID: a.ID, ID: os[0].ID}); o.TotalCents != 9000+30000 || o.Status != orderSold {
		t.Errorf("pedido ajustado: total %d estado %d", o.TotalCents, o.Status)
	}
	if rec := do(h, "GET", aHost+"/panel/ventas/resumen", nil, aCookie); rec.Code != 200 || !strings.Contains(rec.Body.String(), "Churros") {
		t.Errorf("resumen: status %d", rec.Code)
	}

	// Ajuste rápido de existencias desde la lista; otra tienda recibe 404.
	if rec := do(h, "POST", aHost+"/panel/productos/"+strconv.FormatInt(blusa, 10)+"/existencias", url.Values{"stock-M|": {"7"}}, aCookie); rec.Code != http.StatusSeeOther {
		t.Errorf("ajustar existencia: status %d", rec.Code)
	}
	if rec := do(h, "POST", bHost+"/panel/productos/"+strconv.FormatInt(churros, 10)+"/existencias", url.Values{"stock": {"99"}}, bCookie); rec.Code != http.StatusNotFound {
		t.Errorf("B ajusta existencia de A: status %d, quiero 404", rec.Code)
	}
	if c, m := stockOf(); c != 250 || m != 7000 {
		t.Errorf("tras ajustar: churros %d, blusa M %d; quiero 250 y 7000", c, m)
	}

	// Límite: 10 pedidos por hora por tienda e IP.
	codes := 0
	for i := range 12 {
		if order(aHost, "198.51.100.9", `{"code":"LMN`+string("ABCDEFGHJKLM"[i])+`22","items":[`+item(churros, "", 250)+`]}`) == http.StatusTooManyRequests {
			codes++
		}
	}
	if codes != 2 {
		t.Errorf("pedidos frenados = %d, quiero 2", codes)
	}
}

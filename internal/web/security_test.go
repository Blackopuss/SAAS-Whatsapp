package web

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/auth"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
)

func TestCardStock(t *testing.T) {
	for _, c := range []struct {
		soldOut          bool
		avail, step, out int32
	}{
		{false, -1, 1000, -1},         // sin inventario
		{false, 5000, 1000, 5000},     // exacto hasta 99 pasos
		{false, 5000000, 1000, 99000}, // arriba se recorta
		{true, 7321000, 1000, 0},      // agotado a mano no publica lo que queda
		{false, 30000, 250, 24750},    // tope en pasos, no en piezas
	} {
		if got := cardStock(c.soldOut, c.avail, c.step); got != c.out {
			t.Errorf("cardStock(%v, %d, %d) = %d, quiero %d", c.soldOut, c.avail, c.step, got, c.out)
		}
	}
}

// doFrom: como do, pero desde otra IP (los límites cuentan por IP).
func doFrom(h http.Handler, ip, method, target string, form url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.RemoteAddr = ip + ":5555"
	for _, c := range cookies {
		r.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func cookieNamed(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// TestSecurityFixes: correcciones de la auditoría 2026-11-24 contra Postgres real.
func TestSecurityFixes(t *testing.T) {
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
	a, b := mk("sec-a-"+suffix), mk("sec-b-"+suffix)
	key := bytes.Repeat([]byte{7}, 32)
	var sent []string
	h := Routes(Deps{
		BaseDomain: "tiendas.test", DB: pool, Q: q, TOTPKey: key,
		Tx: func(ctx context.Context, fn func(db.Querier) error) error { return db.InTx(ctx, pool, fn) },
		SendReset: func(_ context.Context, _, link string) error {
			sent = append(sent, link)
			return nil
		},
	})
	aHost := "http://" + a.Slug + ".tiendas.test"
	seller := login(t, h, a.Slug+".tiendas.test", a.Slug+"@x.mx")
	product := func() db.Product {
		t.Helper()
		ps, err := q.ListProducts(ctx, a.ID)
		if err != nil || len(ps) != 1 {
			t.Fatalf("productos: %v %d", err, len(ps))
		}
		return ps[0]
	}

	// Precio: la vendedora lo pone al crear; al editar, el campo price se ignora y la oferta sigue siendo suya.
	if rec := do(h, "POST", aHost+"/panel/productos", url.Values{"name": {"Blusa"}, "price": {"300"}, "unit": {"pieza"}, "step": {"1"}}, seller); rec.Code != http.StatusSeeOther {
		t.Fatalf("crear: status %d", rec.Code)
	}
	p := product()
	pid := strconv.FormatInt(p.ID, 10)
	if p.PriceCents != 30000 {
		t.Fatalf("precio al crear: %d", p.PriceCents)
	}
	edit := url.Values{"name": {"Blusa"}, "price": {"1"}, "sale_price": {"250"}, "unit": {"pieza"}, "step": {"1"}}
	if rec := do(h, "POST", aHost+"/panel/productos/"+pid, edit, seller); rec.Code != http.StatusSeeOther {
		t.Fatalf("editar: status %d", rec.Code)
	}
	if p = product(); p.PriceCents != 30000 || !p.OnSale || p.SalePriceCents.Int32 != 25000 {
		t.Fatalf("la vendedora cambió el precio normal o perdió la oferta: %+v", p)
	}
	edit.Set("sale_price", "400") // la oferta se compara con el precio guardado, no con el del formulario
	edit.Set("price", "1000")
	if rec := do(h, "POST", aHost+"/panel/productos/"+pid, edit, seller); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("oferta arriba del precio guardado: status %d", rec.Code)
	}

	// Superadmin cambia el precio; queda en la bitácora y la oferta que ya no queda abajo se quita.
	secret := auth.NewTOTPSecret()
	sealed, _ := auth.Seal(key, secret)
	adminEmail := "admin-sec-" + suffix + "@x.mx"
	adminPass, _ := auth.Hash("admin-contraseña-larga")
	adminID, err := q.CreateAdmin(ctx, db.CreateAdminParams{Email: adminEmail, PasswordHash: adminPass, TotpSecret: sealed})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DELETE FROM admins WHERE id = $1", adminID) })
	adminURL := "http://admin.tiendas.test"
	rec := do(h, "POST", adminURL+"/login", url.Values{"email": {adminEmail}, "password": {"admin-contraseña-larga"}, "code": {auth.TOTPCode(secret, time.Now().Unix()/30)}}, nil)
	admin := cookieNamed(rec, "tiendas_admin")
	if rec.Code != http.StatusSeeOther || admin == nil {
		t.Fatalf("admin login: status %d", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "no-store") {
		t.Errorf("admin sin no-store: %q", got)
	}
	priceURL := adminURL + "/tiendas/" + strconv.FormatInt(a.ID, 10) + "/productos/" + pid + "/precio"
	do(h, "POST", priceURL, url.Values{"price": {"200"}}, nil) // sin sesión de admin
	if p = product(); p.PriceCents != 30000 {
		t.Fatalf("cambió el precio sin sesión de admin: %d", p.PriceCents)
	}
	if rec := do(h, "POST", aHost+"/tiendas/"+strconv.FormatInt(a.ID, 10)+"/productos/"+pid+"/precio", url.Values{"price": {"200"}}, seller); rec.Code == http.StatusSeeOther {
		t.Fatal("la ruta de precio existe en el host de la tienda")
	}
	wrongStore := adminURL + "/tiendas/" + strconv.FormatInt(b.ID, 10) + "/productos/" + pid + "/precio"
	if rec := do(h, "POST", wrongStore, url.Values{"price": {"200"}}, admin); rec.Code != http.StatusNotFound {
		t.Fatalf("producto de otra tienda: status %d", rec.Code)
	}
	if rec := do(h, "POST", priceURL, url.Values{"price": {"200"}}, admin); rec.Code != http.StatusSeeOther {
		t.Fatalf("admin cambia precio: status %d", rec.Code)
	}
	if p = product(); p.PriceCents != 20000 || p.OnSale || p.SalePriceCents.Valid {
		t.Fatalf("precio del admin: %+v", p)
	}
	var audits int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM admin_audit WHERE store_id = $1 AND action = 'precio cambiado' AND detail LIKE 'Blusa:%'", a.ID).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("bitácora de precio: %d %v", audits, err)
	}

	// Caché: el 303 de una visita sin sesión a una ruta .csv no se puede guardar en Cloudflare.
	if rec := do(h, "GET", aHost+"/panel/ventas/ventas.csv", nil, nil); rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Cache-Control"), "no-store") {
		t.Fatalf("csv anónimo: status %d Cache-Control %q", rec.Code, rec.Header().Get("Cache-Control"))
	}

	// Bloqueo: 5 fallos desde otra IP bloquean el email; recuperar la contraseña recuerda ese navegador.
	bHost := "http://" + b.Slug + ".tiendas.test"
	bEmail := b.Slug + "@x.mx"
	for i := range 5 {
		doFrom(h, "198.51.100."+strconv.Itoa(10+i), "POST", bHost+"/panel/login", url.Values{"email": {bEmail}, "password": {"mala"}})
	}
	rec = doFrom(h, "192.0.2.50", "POST", bHost+"/panel/login", url.Values{"email": {bEmail}, "password": {"contraseña-segura"}})
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "900" {
		t.Fatalf("email bloqueado: status %d Retry-After %q", rec.Code, rec.Header().Get("Retry-After"))
	}
	if rec := doFrom(h, "192.0.2.50", "POST", bHost+"/panel/recuperar", url.Values{"email": {bEmail}}); rec.Code != http.StatusOK || len(sent) != 1 {
		t.Fatalf("pedir enlace: status %d envíos %d", rec.Code, len(sent))
	}
	link, _ := url.Parse(sent[0])
	rec = doFrom(h, "192.0.2.50", "POST", bHost+"/panel/recuperar?t="+url.QueryEscape(link.Query().Get("t")), url.Values{"password": {"nueva-contraseña-1"}, "confirm": {"nueva-contraseña-1"}})
	device := cookieNamed(rec, deviceCookie)
	if rec.Code != http.StatusOK || device == nil || !device.HttpOnly || device.Path != "/panel" {
		t.Fatalf("recuperar: status %d cookie %+v", rec.Code, device)
	}
	newPass := url.Values{"email": {bEmail}, "password": {"nueva-contraseña-1"}}
	if rec := doFrom(h, "192.0.2.51", "POST", bHost+"/panel/login", newPass); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("otro navegador con el email bloqueado: status %d", rec.Code)
	}
	if rec := doFrom(h, "192.0.2.50", "POST", bHost+"/panel/login", newPass, device); rec.Code != http.StatusSeeOther {
		t.Fatalf("el navegador que recuperó no entra: status %d", rec.Code)
	}
}

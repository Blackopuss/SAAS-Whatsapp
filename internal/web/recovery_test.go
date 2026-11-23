package web

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/auth"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
)

// TestPasswordRecoveryAndAdmin: enlace de un solo uso, ligado a su tienda, que cierra sesiones viejas;
// superadmin con TOTP que no acepta repetir un código. Postgres real.
func TestPasswordRecoveryAndAdmin(t *testing.T) {
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
	a, b := mk("rec-a-"+suffix), mk("rec-b-"+suffix)

	key := bytes.Repeat([]byte{9}, 32)
	var sent []string
	h := Routes(Deps{
		BaseDomain: "tiendas.test", DB: pool, Q: q, TOTPKey: key,
		Tx: func(ctx context.Context, fn func(db.Querier) error) error { return db.InTx(ctx, pool, fn) },
		SendReset: func(_ context.Context, to, link string) error {
			sent = append(sent, link)
			return nil
		},
	})
	aHost := a.Slug + ".tiendas.test"
	oldSession := login(t, h, aHost, a.Slug+"@x.mx")

	// Email inexistente y existente dan la misma respuesta; solo el real recibe enlace.
	r1 := do(h, "POST", "http://"+aHost+"/panel/recuperar", url.Values{"email": {"nadie@x.mx"}}, nil)
	r2 := do(h, "POST", "http://"+aHost+"/panel/recuperar", url.Values{"email": {a.Slug + "@x.mx"}}, nil)
	if r1.Code != 200 || r2.Code != 200 || r1.Body.String() != r2.Body.String() || len(sent) != 1 {
		t.Fatalf("respuestas distintas o envíos=%d", len(sent))
	}
	link, _ := url.Parse(sent[0])
	if link.Host != aHost || link.Path != "/panel/recuperar" {
		t.Fatalf("enlace: %s", sent[0])
	}
	tok := link.Query().Get("t")

	// El token de A no sirve en la tienda B.
	if rec := do(h, "GET", "http://"+b.Slug+".tiendas.test/panel/recuperar?t="+tok, nil, nil); rec.Code != http.StatusGone {
		t.Errorf("token de A en B: status %d", rec.Code)
	}
	if rec := do(h, "POST", link.String(), url.Values{"password": {"corta"}, "confirm": {"corta"}}, nil); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("contraseña corta: status %d", rec.Code)
	}
	time.Sleep(1100 * time.Millisecond) // el cambio debe caer en un segundo posterior al login viejo
	newPass := url.Values{"password": {"otra-contraseña-1"}, "confirm": {"otra-contraseña-1"}}
	if rec := do(h, "POST", link.String(), newPass, nil); rec.Code != http.StatusOK {
		t.Fatalf("cambiar contraseña: status %d", rec.Code)
	}
	if rec := do(h, "POST", link.String(), newPass, nil); rec.Code != http.StatusGone {
		t.Errorf("token reusado: status %d", rec.Code)
	}
	if rec := do(h, "GET", "http://"+aHost+"/panel/", nil, oldSession); rec.Code != http.StatusSeeOther {
		t.Errorf("sesión previa al cambio sigue abierta: status %d", rec.Code)
	}
	if rec := do(h, "POST", "http://"+aHost+"/panel/login", url.Values{"email": {a.Slug + "@x.mx"}, "password": {"otra-contraseña-1"}}, nil); rec.Code != http.StatusSeeOther {
		t.Errorf("login con la nueva contraseña: status %d", rec.Code)
	}

	// Superadmin.
	secret := auth.NewTOTPSecret()
	sealed, _ := auth.Seal(key, secret)
	adminEmail := "admin-" + suffix + "@x.mx"
	adminPass, _ := auth.Hash("admin-contraseña-larga")
	adminID, err := q.CreateAdmin(ctx, db.CreateAdminParams{Email: adminEmail, PasswordHash: adminPass, TotpSecret: sealed})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DELETE FROM admins WHERE id = $1", adminID) })
	adminURL := "http://admin.tiendas.test"
	if rec := do(h, "GET", adminURL+"/", nil, nil); rec.Code != http.StatusSeeOther {
		t.Errorf("admin sin sesión: status %d", rec.Code)
	}
	code := auth.TOTPCode(secret, time.Now().Unix()/30)
	form := url.Values{"email": {adminEmail}, "password": {"admin-contraseña-larga"}, "code": {"000000"}}
	if code == "000000" {
		form.Set("code", "111111")
	}
	if rec := do(h, "POST", adminURL+"/login", form, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("admin con código malo: status %d", rec.Code)
	}
	form.Set("code", code)
	rec := do(h, "POST", adminURL+"/login", form, nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("admin login: status %d", rec.Code)
	}
	var adminCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "tiendas_admin" {
			adminCookie = c
		}
	}
	if adminCookie == nil || adminCookie.SameSite != http.SameSiteStrictMode || !adminCookie.HttpOnly {
		t.Fatalf("cookie admin: %+v", adminCookie)
	}
	if rec := do(h, "POST", adminURL+"/login", form, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("código TOTP reusado: status %d", rec.Code)
	}
	// La cookie del admin no abre el panel de una tienda.
	if rec := do(h, "GET", "http://"+aHost+"/panel/", nil, adminCookie); rec.Code != http.StatusSeeOther {
		t.Errorf("cookie admin en panel de tienda: status %d", rec.Code)
	}
	rec = do(h, "GET", adminURL+"/", nil, adminCookie)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), a.Slug) {
		t.Fatalf("lista de tiendas: status %d", rec.Code)
	}
	rec = do(h, "POST", adminURL+"/tiendas/"+strconv.FormatInt(b.ID, 10)+"/recuperar", url.Values{}, adminCookie)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "http://"+b.Slug+".tiendas.test/panel/recuperar?t=") {
		t.Fatalf("enlace desde admin: status %d", rec.Code)
	}

	// Alta de tienda desde el admin: crea tienda, dueña y suscripción, y da el enlace para su contraseña.
	newSlug := "alta-" + suffix
	rec = do(h, "POST", adminURL+"/tiendas", url.Values{"slug": {newSlug}, "name": {"Alta Moda"}, "email": {newSlug + "@x.mx"}, "whatsapp": {"55 1111 2222"}, "price": {"149"}, "founder": {"on"}}, adminCookie)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), newSlug+".tiendas.test/panel/recuperar?t=") || !strings.Contains(rec.Body.String(), "https://wa.me/525511112222") {
		t.Fatalf("alta de tienda: status %d", rec.Code)
	}
	ns, err := q.GetStoreBySlug(ctx, newSlug)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DELETE FROM stores WHERE id = $1", ns.ID) })
	if rec := do(h, "POST", adminURL+"/tiendas", url.Values{"slug": {newSlug}, "name": {"Otra"}, "email": {"o@x.mx"}, "price": {"149"}}, adminCookie); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("subdominio repetido: status %d", rec.Code)
	}
	nsID := strconv.FormatInt(ns.ID, 10)

	// Pago: la fecha avanza y la racha cuenta.
	today, _ := q.Today(ctx)
	pay := url.Values{"amount": {"149"}, "months": {"3"}, "paid_on": {today.Time.Format("2006-01-02")}}
	if rec := do(h, "POST", adminURL+"/tiendas/"+nsID+"/pagos", pay, adminCookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("registrar pago: status %d", rec.Code)
	}
	sub, _ := q.GetSubscription(ctx, ns.ID)
	if !sub.PaidUntil.Valid || !sub.PaidUntil.Time.Equal(today.Time.AddDate(0, 3, 0)) || sub.StreakMonths != 3 || !sub.IsFounder || sub.PriceCents != 14900 {
		t.Errorf("suscripción tras pago: %+v", sub)
	}
	future := url.Values{"amount": {"149"}, "months": {"1"}, "paid_on": {today.Time.AddDate(0, 0, 2).Format("2006-01-02")}}
	if rec := do(h, "POST", adminURL+"/tiendas/"+nsID+"/pagos", future, adminCookie); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("pago con fecha futura: status %d", rec.Code)
	}

	// Pausa manual: la tienda pública no revela nada de la vendedora; el panel sigue abierto.
	nsHost := "http://" + newSlug + ".tiendas.test"
	if rec := do(h, "POST", adminURL+"/tiendas/"+nsID+"/pausar", url.Values{"reason": {"prueba"}}, adminCookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("pausar: status %d", rec.Code)
	}
	for _, path := range []string{"/", "/carrito.json", "/preguntas"} {
		rec := do(h, "GET", nsHost+path, nil, nil)
		if rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), "Alta Moda") || strings.Contains(rec.Body.String(), "5511112222") {
			t.Errorf("tienda en pausa %s: status %d", path, rec.Code)
		}
	}
	if rec := do(h, "GET", nsHost+"/panel/login", nil, nil); rec.Code != 200 {
		t.Errorf("panel de tienda en pausa: status %d", rec.Code)
	}
	if rec := do(h, "POST", adminURL+"/tiendas/"+nsID+"/reactivar", url.Values{}, adminCookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("reactivar: status %d", rec.Code)
	}
	if rec := do(h, "GET", nsHost+"/", nil, nil); rec.Code != 200 {
		t.Errorf("tienda reactivada: status %d", rec.Code)
	}

	// Corte automático: vencida hace 6 días se pausa; un pago al corriente la reabre.
	if _, err := pool.Exec(ctx, "UPDATE subscriptions SET paid_until = $2 WHERE store_id = $1", ns.ID, today.Time.AddDate(0, 0, -6)); err != nil {
		t.Fatal(err)
	}
	cut, err := q.SuspendOverdue(ctx)
	if err != nil || !slices.ContainsFunc(cut, func(r db.SuspendOverdueRow) bool { return r.ID == ns.ID }) {
		t.Fatalf("corte automático: %v %v", cut, err)
	}
	if rec := do(h, "POST", adminURL+"/tiendas/"+nsID+"/pagos", pay, adminCookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("pago tras corte: status %d", rec.Code)
	}
	if s, _ := q.GetStoreAdmin(ctx, ns.ID); s.Status != "active" || s.StreakMonths != 3 {
		t.Errorf("tras pagar atrasado: estado %s racha %d (quiero active y racha reiniciada a 3)", s.Status, s.StreakMonths)
	}

	// Bitácora: cada acción con quién la hizo.
	rec = do(h, "GET", adminURL+"/bitacora", nil, adminCookie)
	for _, action := range []string{"alta de tienda", "pago registrado", "pausada", "reactivada", adminEmail} {
		if !strings.Contains(rec.Body.String(), action) {
			t.Errorf("bitácora sin %q", action)
		}
	}
}

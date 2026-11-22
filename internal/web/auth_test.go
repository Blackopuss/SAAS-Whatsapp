package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/auth"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
)

// fakeQ implementa solo lo que usan los tests de auth; el resto de db.Querier entra en pánico si se llama.
type fakeQ struct {
	db.Querier
	stores map[string]db.Store
	users  map[db.GetUserForLoginParams]db.GetUserForLoginRow
}

func newFakeQ() *fakeQ {
	hash, err := auth.Hash("contraseña-segura")
	if err != nil {
		panic(err)
	}
	return &fakeQ{
		stores: map[string]db.Store{"ana": {ID: 1, Slug: "ana", Name: "Ana Moda", Status: "active"}, "luz": {ID: 2, Slug: "luz", Name: "Paca Luz", Status: "active"}},
		users: map[db.GetUserForLoginParams]db.GetUserForLoginRow{
			{StoreID: 1, Email: "ana@x.mx"}: {ID: 10, StoreID: 1, PasswordHash: hash},
			{StoreID: 2, Email: "luz@x.mx"}: {ID: 20, StoreID: 2, PasswordHash: hash},
		},
	}
}

func (f *fakeQ) GetStoreBySlug(_ context.Context, slug string) (db.Store, error) {
	if s, ok := f.stores[slug]; ok {
		return s, nil
	}
	return db.Store{}, pgx.ErrNoRows
}

func (f *fakeQ) GetUserForLogin(_ context.Context, p db.GetUserForLoginParams) (db.GetUserForLoginRow, error) {
	if u, ok := f.users[p]; ok {
		return u, nil
	}
	return db.GetUserForLoginRow{}, pgx.ErrNoRows
}

func do(h http.Handler, method, target string, form url.Values, cookie *http.Cookie) *httptest.ResponseRecorder {
	var r *http.Request
	if form != nil {
		r = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	r.RemoteAddr = "203.0.113.7:5555"
	if cookie != nil {
		r.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func login(t *testing.T, h http.Handler, host, email string) *http.Cookie {
	t.Helper()
	rec := do(h, "POST", "http://"+host+"/panel/login", url.Values{"email": {email}, "password": {"contraseña-segura"}}, nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("login %s en %s: status %d", email, host, rec.Code)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "tiendas_sid" {
			if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/panel" {
				t.Fatalf("cookie sin flags esperados: %+v", c)
			}
			return c
		}
	}
	t.Fatal("login sin cookie de sesión")
	return nil
}

func TestTenantRouting(t *testing.T) {
	h := testRoutes(nil)
	for target, want := range map[string]int{
		"http://ana.tiendas.test/":       200,
		"http://ANA.tiendas.test:8080/":  200,
		"http://nadie.tiendas.test/":     404,
		"http://admin.tiendas.test/":     404, // reservado
		"http://x.ana.tiendas.test/":     404,
		"http://otro-dominio.com/":       404,
		"http://ana.tiendas.test/panel/": 303, // sin sesión -> login
	} {
		if got := do(h, "GET", target, nil, nil).Code; got != want {
			t.Errorf("%s: status %d, quiero %d", target, got, want)
		}
	}
}

func TestLoginAndIsolation(t *testing.T) {
	h := testRoutes(nil)
	anaCookie := login(t, h, "ana.tiendas.test", "ana@x.mx")

	if rec := do(h, "GET", "http://ana.tiendas.test/panel/", nil, anaCookie); rec.Code != 200 || !strings.Contains(rec.Body.String(), "Ana Moda") {
		t.Fatalf("ana no entra a su panel: %d", rec.Code)
	}
	// La sesión de ana no abre el panel de luz.
	if rec := do(h, "GET", "http://luz.tiendas.test/panel/", nil, anaCookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("sesión de ana abrió el panel de luz: %d", rec.Code)
	}
	// Las credenciales de luz no sirven en ana.
	rec := do(h, "POST", "http://ana.tiendas.test/panel/login", url.Values{"email": {"luz@x.mx"}, "password": {"contraseña-segura"}}, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("credenciales de luz entraron a ana: %d", rec.Code)
	}
	// Logout invalida la sesión.
	do(h, "POST", "http://ana.tiendas.test/panel/logout", url.Values{}, anaCookie)
	if rec := do(h, "GET", "http://ana.tiendas.test/panel/", nil, anaCookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("la sesión sigue viva después de salir: %d", rec.Code)
	}
}

func TestLoginSameErrorMessage(t *testing.T) {
	h := testRoutes(nil)
	wrongPass := do(h, "POST", "http://ana.tiendas.test/panel/login", url.Values{"email": {"ana@x.mx"}, "password": {"mala"}}, nil)
	noUser := do(h, "POST", "http://ana.tiendas.test/panel/login", url.Values{"email": {"nadie@x.mx"}, "password": {"mala"}}, nil)
	if wrongPass.Code != 401 || noUser.Code != 401 {
		t.Fatalf("status %d / %d", wrongPass.Code, noUser.Code)
	}
	if !strings.Contains(wrongPass.Body.String(), loginFailMsg) || !strings.Contains(noUser.Body.String(), loginFailMsg) {
		t.Fatal("los mensajes de error deben ser idénticos")
	}
}

func TestLoginRateLimit(t *testing.T) {
	h := testRoutes(nil)
	for i := range 5 {
		if rec := do(h, "POST", "http://ana.tiendas.test/panel/login", url.Values{"email": {"ana@x.mx"}, "password": {"mala"}}, nil); rec.Code != 401 {
			t.Fatalf("intento %d: status %d", i+1, rec.Code)
		}
	}
	// El sexto intento se bloquea aunque la contraseña sea correcta.
	rec := do(h, "POST", "http://ana.tiendas.test/panel/login", url.Values{"email": {"ana@x.mx"}, "password": {"contraseña-segura"}}, nil)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("sexto intento: status %d", rec.Code)
	}
}

func TestCSRFCrossOriginRejected(t *testing.T) {
	h := testRoutes(nil)
	r := httptest.NewRequest("POST", "http://ana.tiendas.test/panel/login", strings.NewReader("email=ana%40x.mx&password=x"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("POST cross-site: status %d", rec.Code)
	}
}

func (f *fakeQ) ListProducts(context.Context, int64) ([]db.Product, error) { return nil, nil }

func (f *fakeQ) GetUserPasswordChangedAt(context.Context, db.GetUserPasswordChangedAtParams) (pgtype.Timestamptz, error) {
	return pgtype.Timestamptz{Valid: true}, nil
}

func (f *fakeQ) ListCategories(context.Context, int64) ([]db.Category, error) { return nil, nil }

func (f *fakeQ) ListStoreProducts(context.Context, int64) ([]db.ListStoreProductsRow, error) {
	return nil, nil
}

func (f *fakeQ) GetSubscription(context.Context, int64) (db.Subscription, error) {
	return db.Subscription{}, pgx.ErrNoRows
}

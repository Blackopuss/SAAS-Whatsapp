// Package web arma el router HTTP y sus middlewares.
package web

import (
	"context"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/alexedwards/scs/v2"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/web/views"
)

type Pinger interface{ Ping(context.Context) error }

// Queries es lo que la capa web usa de sqlc; *db.Queries lo cumple y los tests lo pueden simular.
type Queries interface {
	GetStoreBySlug(context.Context, string) (db.Store, error)
	GetUserForLogin(context.Context, db.GetUserForLoginParams) (db.GetUserForLoginRow, error)
}

type Deps struct {
	BaseDomain string
	Secure     bool      // cookies Secure; false solo en desarrollo por http
	DB         Pinger
	Q          Queries
	Sessions   scs.Store // nil = memoria (tests)
}

type app struct {
	q        Queries
	sessions *scs.SessionManager
	limit    *limiter
}

func newSessions(store scs.Store, secure bool) *scs.SessionManager {
	sm := scs.New()
	if store != nil {
		sm.Store = store
	}
	sm.IdleTimeout = 7 * 24 * time.Hour
	sm.Lifetime = 30 * 24 * time.Hour
	sm.Cookie.Name = "tiendas_sid"
	sm.Cookie.Path = "/panel" // la tienda pública no recibe la cookie
	sm.Cookie.HttpOnly = true
	sm.Cookie.Secure = secure
	sm.Cookie.SameSite = http.SameSiteLaxMode
	sm.Cookie.Persist = true
	return sm
}

// Orden del plan: recover, request_id, logger, security headers, tenant, sesión, CSRF.
func Routes(d Deps) http.Handler {
	a := &app{q: d.Q, sessions: newSessions(d.Sessions, d.Secure), limit: newLimiter(5, 15*time.Minute)}
	t := &tenants{m: map[string]cachedStore{}, lookup: d.Q.GetStoreBySlug, base: d.BaseDomain}

	root := http.NewServeMux()
	root.HandleFunc("GET /{$}", render(func(*http.Request) templ.Component { return views.Hello() }))

	panel := http.NewServeMux()
	panel.HandleFunc("GET /panel/login", a.loginForm)
	panel.HandleFunc("POST /panel/login", a.loginPost)
	panel.HandleFunc("POST /panel/logout", a.logout)
	panel.Handle("GET /panel/{$}", a.requireUser(http.HandlerFunc(a.panelHome)))

	store := http.NewServeMux()
	store.HandleFunc("GET /{$}", render(func(r *http.Request) templ.Component { return views.StoreHome(StoreFrom(r.Context()).Name) }))
	store.Handle("/panel/", chain(panel, a.sessions.LoadAndSave, http.NewCrossOriginProtection().Handler))

	top := http.NewServeMux()
	top.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	top.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := d.DB.Ping(r.Context()); err != nil {
			Log(r.Context()).Error("healthz", "err", err)
			http.Error(w, "db no disponible", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
	tenantHandler := t.middleware(store)
	top.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, isRoot := slugFromHost(r.Host, d.BaseDomain); isRoot {
			root.ServeHTTP(w, r)
			return
		}
		tenantHandler.ServeHTTP(w, r)
	}))

	return chain(top, recoverer, requestID, logger, securityHeaders)
}

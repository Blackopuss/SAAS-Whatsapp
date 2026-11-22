// Package web arma el router HTTP y sus middlewares.
package web

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/alexedwards/scs/v2"
	"github.com/jackc/pgx/v5"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/storage"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/web/views"
)

type Pinger interface{ Ping(context.Context) error }

type Deps struct {
	BaseDomain string
	Secure     bool // cookies Secure; false solo en desarrollo por http
	DB         Pinger
	Q          db.Querier
	Tx         func(context.Context, func(db.Querier) error) error // nil = sin transacción (tests con fakes)
	Sessions   scs.Store                                           // nil = memoria (tests)
	Storage    storage.Storage
	MediaURL   string // prefijo público de las fotos, p. ej. "/media/"
	MediaDir   string // si no está vacío, se sirve en /media/ (solo desarrollo)
}

type app struct {
	q        db.Querier
	tx       func(context.Context, func(db.Querier) error) error
	sessions *scs.SessionManager
	limit    *limiter
	store    storage.Storage
	mediaURL string
}

func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

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
	a := &app{q: d.Q, tx: d.Tx, sessions: newSessions(d.Sessions, d.Secure), limit: newLimiter(5, 15*time.Minute), store: d.Storage, mediaURL: d.MediaURL}
	if a.tx == nil {
		a.tx = func(_ context.Context, fn func(db.Querier) error) error { return fn(d.Q) }
	}
	t := &tenants{m: map[string]cachedStore{}, lookup: d.Q.GetStoreBySlug, base: d.BaseDomain}

	root := http.NewServeMux()
	root.HandleFunc("GET /{$}", render(func(*http.Request) templ.Component { return views.Hello() }))

	panel := http.NewServeMux()
	panel.HandleFunc("GET /panel/login", a.loginForm)
	panel.HandleFunc("POST /panel/login", a.loginPost)
	panel.HandleFunc("POST /panel/logout", a.logout)
	for pattern, h := range map[string]http.HandlerFunc{
		"GET /panel/{$}":                            a.products,
		"GET /panel/productos/nuevo":                a.productNew,
		"POST /panel/productos":                     a.productSave,
		"GET /panel/productos/{id}":                 a.productEdit,
		"POST /panel/productos/{id}":                a.productSave,
		"POST /panel/productos/{id}/borrar":         a.productDelete,
		"POST /panel/productos/{id}/marcar/{flag}":  a.productToggle,
		"POST /panel/productos/{id}/fotos":          a.photoUpload,
		"POST /panel/fotos/{id}/borrar":             a.photoDelete,
		"GET /panel/categorias":                     a.categories,
		"POST /panel/categorias":                    a.categoryCreate,
		"POST /panel/categorias/{id}/renombrar":     a.categoryRename,
		"POST /panel/categorias/{id}/borrar":        a.categoryDelete,
		"POST /panel/categorias/{id}/mover":         a.categoryMove,
	} {
		panel.Handle(pattern, a.requireUser(h))
	}

	store := http.NewServeMux()
	store.HandleFunc("GET /{$}", render(func(r *http.Request) templ.Component { return views.StoreHome(StoreFrom(r.Context()).Name) }))
	store.Handle("/panel/", chain(panel, a.sessions.LoadAndSave, http.NewCrossOriginProtection().Handler))

	top := http.NewServeMux()
	top.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	if d.MediaDir != "" {
		media := http.StripPrefix("/media/", http.FileServer(http.Dir(d.MediaDir)))
		top.Handle("GET /media/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable") // llaves aleatorias: nunca cambian
			media.ServeHTTP(w, r)
		}))
	}
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

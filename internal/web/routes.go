// Package web arma el router HTTP y sus middlewares.
package web

import (
	"context"
	"errors"
	"net/http"
	"os"
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
	TOTPKey    []byte // 32 bytes; sin ella no se monta admin.<dominio>
	// SendReset entrega el enlace de recuperación; nil = la vendedora lo pide a soporte (admin).
	SendReset func(ctx context.Context, to, link string) error
}

type app struct {
	q        db.Querier
	tx       func(context.Context, func(db.Querier) error) error
	sessions *scs.SessionManager
	limit    *limiter
	// orderLimit: pedidos por tienda e IP; frena a quien llene de pedidos falsos el panel de una vendedora.
	orderLimit *limiter
	// accounts: fallos por cuenta (email). Aparte de limit porque nunca suelta una llave viva para hacer lugar.
	accounts *limiter
	// failAlarm: cuenta logins fallidos de todo el servidor para levantar una alerta.
	failAlarm *limiter
	store     storage.Storage
	mediaURL  string

	base      string
	secure    bool
	admin     *scs.SessionManager
	totpKey   []byte
	sendReset func(ctx context.Context, to, link string) error
	tenants   *tenants
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
	a := &app{q: d.Q, tx: d.Tx, sessions: newSessions(d.Sessions, d.Secure), limit: newLimiter(5, 15*time.Minute), orderLimit: newLimiter(10, time.Hour),
		accounts: newStrictLimiter(5, 15*time.Minute), failAlarm: newLimiter(failAlarmMax, failAlarmWindow), store: d.Storage, mediaURL: d.MediaURL,
		base: d.BaseDomain, secure: d.Secure, totpKey: d.TOTPKey, sendReset: d.SendReset}
	if a.tx == nil {
		a.tx = func(_ context.Context, fn func(db.Querier) error) error { return fn(d.Q) }
	}
	t := &tenants{m: map[string]cachedStore{}, lookup: d.Q.GetStoreBySlug, base: d.BaseDomain}
	a.tenants = t

	root := http.NewServeMux()
	root.HandleFunc("GET /{$}", render(func(*http.Request) templ.Component { return views.Hello() }))

	panel := http.NewServeMux()
	panel.HandleFunc("GET /panel/login", a.loginForm)
	panel.HandleFunc("POST /panel/login", a.loginPost)
	panel.HandleFunc("POST /panel/logout", a.logout)
	panel.HandleFunc("GET /panel/recuperar", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("t") {
			a.resetForm(w, r)
			return
		}
		a.forgotForm(w, r)
	})
	panel.HandleFunc("POST /panel/recuperar", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("t") {
			a.resetPost(w, r)
			return
		}
		a.forgotPost(w, r)
	})
	for pattern, h := range map[string]http.HandlerFunc{
		"GET /panel/{$}":                           a.products,
		"GET /panel/productos/nuevo":               a.productNew,
		"POST /panel/productos":                    a.productSave,
		"GET /panel/productos/{id}":                a.productEdit,
		"POST /panel/productos/{id}":               a.productSave,
		"POST /panel/productos/{id}/borrar":        a.productDelete,
		"POST /panel/productos/{id}/marcar/{flag}": a.productToggle,
		"POST /panel/productos/{id}/fotos":         a.photoUpload,
		"POST /panel/fotos/{id}/borrar":            a.photoDelete,
		"GET /panel/tienda":                        a.myStore,
		"POST /panel/tienda":                       a.myStoreSave,
		"GET /panel/tienda/vista":                  a.myStorePreview,
		"GET /panel/tienda/qr.png":                 a.myStoreQR,
		"POST /panel/tienda/logo":                  a.logoUpload,
		"POST /panel/tienda/logo/borrar":           a.logoDelete,
		"GET /panel/mensajes":                      a.messages,
		"GET /panel/ventas":                        a.sales,
		"GET /panel/ventas/resumen":                a.salesSummary,
		"GET /panel/ventas/ventas.csv":             a.salesCSV,
		"POST /panel/ventas/{id}/vender":           a.orderSell,
		"POST /panel/ventas/{id}/no-vender":        a.orderLose,
		"POST /panel/ventas/{id}/deshacer":         a.orderUndo,
		"POST /panel/ventas/{id}/ajustar":          a.orderAdjust,
		"POST /panel/productos/{id}/existencias":   a.productStock,
		"GET /panel/categorias":                    a.categories,
		"POST /panel/categorias":                   a.categoryCreate,
		"POST /panel/categorias/{id}/renombrar":    a.categoryRename,
		"POST /panel/categorias/{id}/borrar":       a.categoryDelete,
		"POST /panel/categorias/{id}/mover":        a.categoryMove,
	} {
		panel.Handle(pattern, a.requireUser(h))
	}

	store := http.NewServeMux()
	for pattern, h := range map[string]http.HandlerFunc{
		"GET /{$}":          a.shopHome,
		"GET /c/{id}":       a.shopCategory,
		"GET /p/{id}":       a.shopProduct,
		"GET /preguntas":    a.shopFAQ,
		"GET /carrito.json": a.shopCartJSON,
	} {
		store.Handle(pattern, a.requireOpen(h))
	}
	// Único POST de la tienda pública: mismo filtro de origen que el panel.
	store.Handle("POST /pedido", a.requireOpen(http.NewCrossOriginProtection().Handler(http.HandlerFunc(a.shopOrder))))
	store.Handle("/panel/", chain(panel, a.sessions.LoadAndSave, http.NewCrossOriginProtection().Handler))

	top := http.NewServeMux()
	top.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(noDirs{http.Dir("web/static")})))
	if d.MediaDir != "" {
		media := http.StripPrefix("/media/", http.FileServer(noDirs{http.Dir(d.MediaDir)}))
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
	admin := http.NotFoundHandler()
	if len(d.TOTPKey) == 32 {
		admin = a.adminRoutes(d.Sessions, d.Secure)
	}
	top.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slug, isRoot := slugFromHost(r.Host, d.BaseDomain)
		if isRoot {
			root.ServeHTTP(w, r)
			return
		}
		if slug == "admin" {
			admin.ServeHTTP(w, r)
			return
		}
		tenantHandler.ServeHTTP(w, r)
	}))

	return chain(top, recoverer, requestID, logger, securityHeaders)
}

// noDirs: sirve archivos pero nunca el listado de una carpeta (en /media/ mostraría las llaves de todas las tiendas).
type noDirs struct{ fs http.FileSystem }

func (n noDirs) Open(name string) (http.File, error) {
	f, err := n.fs.Open(name)
	if err != nil {
		return nil, err
	}
	if st, err := f.Stat(); err != nil || st.IsDir() {
		_ = f.Close()
		return nil, os.ErrNotExist
	}
	return f, nil
}

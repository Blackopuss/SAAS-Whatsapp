package web

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/auth"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/web/views"
)

const adminFailMsg = "Email, contraseña o código incorrectos."

// adminRoutes vive en admin.<dominio>: cookie propia, sesión corta y TOTP obligatorio.
func (a *app) adminRoutes(store scs.Store, secure bool) http.Handler {
	sm := scs.New()
	if store != nil {
		sm.Store = store
	}
	sm.IdleTimeout = 30 * time.Minute
	sm.Lifetime = 12 * time.Hour
	sm.Cookie.Name = "tiendas_admin"
	sm.Cookie.HttpOnly = true
	sm.Cookie.Secure = secure
	sm.Cookie.SameSite = http.SameSiteStrictMode
	a.admin = sm

	mux := http.NewServeMux()
	mux.HandleFunc("GET /login", a.adminLoginForm)
	mux.HandleFunc("POST /login", a.adminLoginPost)
	mux.HandleFunc("POST /logout", func(w http.ResponseWriter, r *http.Request) {
		_ = a.admin.Destroy(r.Context())
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
	mux.Handle("GET /{$}", a.requireAdmin(http.HandlerFunc(a.adminStores)))
	mux.Handle("POST /tiendas/{id}/recuperar", a.requireAdmin(http.HandlerFunc(a.adminReset)))
	return chain(mux, sm.LoadAndSave, http.NewCrossOriginProtection().Handler)
}

func (a *app) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.admin.GetInt64(r.Context(), "admin_id") == 0 {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *app) adminLoginForm(w http.ResponseWriter, r *http.Request) {
	renderStatus(w, r, http.StatusOK, views.AdminLogin("", ""))
}

func (a *app) adminLoginPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	email := strings.ToLower(strings.TrimSpace(r.PostFormValue("email")))
	pass := r.PostFormValue("password")
	code := strings.ReplaceAll(r.PostFormValue("code"), " ", "")
	fail := func(status int, msg string) {
		Log(ctx).Warn("admin: login fallido", "ip", clientIP(r))
		renderStatus(w, r, status, views.AdminLogin(email, msg))
	}
	if len(email) > 254 || len(pass) > 1024 || len(code) != 6 {
		fail(http.StatusUnauthorized, adminFailMsg)
		return
	}
	if !a.limit.allow("admin-ip:"+clientIP(r)) || !a.limit.allow("admin-email:"+email) {
		fail(http.StatusTooManyRequests, "Demasiados intentos. Espera 15 minutos y vuelve a intentar.")
		return
	}
	ad, err := a.q.GetAdminByEmail(ctx, email)
	hash := auth.DummyHash
	switch {
	case err == nil:
		hash = ad.PasswordHash
	case !isNoRows(err):
		a.fail(w, r, err)
		return
	}
	if ok, err := auth.Verify(pass, hash); err != nil || !ok || ad.ID == 0 {
		fail(http.StatusUnauthorized, adminFailMsg)
		return
	}
	secret, err := auth.Open(a.totpKey, ad.TotpSecret)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	step, ok := auth.VerifyTOTP(secret, code, time.Now())
	if !ok {
		fail(http.StatusUnauthorized, adminFailMsg)
		return
	}
	// Marca el paso como usado: el mismo código no sirve dos veces.
	if n, err := a.q.UseTOTPStep(ctx, db.UseTOTPStepParams{ID: ad.ID, LastTotpStep: step}); err != nil || n == 0 {
		fail(http.StatusUnauthorized, adminFailMsg)
		return
	}
	if err := a.admin.RenewToken(ctx); err != nil {
		a.fail(w, r, err)
		return
	}
	a.admin.Put(ctx, "admin_id", ad.ID)
	Log(ctx).Info("admin: login", "admin_id", ad.ID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *app) adminStores(w http.ResponseWriter, r *http.Request) {
	stores, err := a.q.ListStoresAdmin(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	renderStatus(w, r, http.StatusOK, views.AdminStores(stores))
}

// adminReset genera un enlace de recuperación para la dueña; el admin se lo manda por WhatsApp.
func (a *app) adminReset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	o, err := a.q.GetStoreOwner(ctx, id)
	if isNoRows(err) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	raw, err := a.issueReset(ctx, id, o.UserID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	link := a.resetLink(r.Host, o.Slug, raw)
	wa := ""
	if o.Whatsapp != "" {
		wa = "https://wa.me/" + strings.TrimPrefix(o.Whatsapp, "+") + "?text=" + url.QueryEscape("Hola, este es tu enlace para crear una nueva contraseña de "+o.Name+" (vence en 30 minutos): "+link)
	}
	Log(ctx).Info("admin: enlace de recuperación", "admin_id", a.admin.GetInt64(ctx, "admin_id"), "store_id", id, "user_id", o.UserID)
	renderStatus(w, r, http.StatusOK, views.AdminResetLink(o.Name, link, wa))
}

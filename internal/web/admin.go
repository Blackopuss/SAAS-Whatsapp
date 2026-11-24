package web

import (
	"net/http"
	"net/url"
	"strconv"
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
	for pattern, h := range map[string]http.HandlerFunc{
		"POST /tiendas/{id}/recuperar":              a.adminReset,
		"GET /tiendas/nueva":                        a.adminStoreNew,
		"POST /tiendas":                             a.adminStoreCreate,
		"GET /tiendas/{id}":                         a.adminStore,
		"POST /tiendas/{id}/pagos":                  a.adminPayment,
		"POST /tiendas/{id}/pausar":                 a.adminSetStatus("paused", "pausada", "pausada"),
		"POST /tiendas/{id}/reactivar":              a.adminSetStatus("active", "reactivada", "reactivada"),
		"POST /tiendas/{id}/plan":                   a.adminPlan,
		"POST /tiendas/{id}/productos/{pid}/precio": a.adminProductPrice,
		"GET /bitacora":                             a.adminAudit,
	} {
		mux.Handle(pattern, a.requireAdmin(h))
	}
	return chain(mux, noStore, sm.LoadAndSave, http.NewCrossOriginProtection().Handler)
}

func (a *app) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		id := a.admin.GetInt64(ctx, "admin_id")
		if id == 0 {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		// Si al admin le cambiaron contraseña y TOTP (o lo borraron), las sesiones de antes ya no sirven.
		changed, err := a.q.GetAdminCredentialsChangedAt(ctx, id)
		if err != nil && !isNoRows(err) {
			a.fail(w, r, err)
			return
		}
		if err != nil || changed.Time.Unix() > a.admin.GetInt64(ctx, "auth_at") {
			_ = a.admin.Destroy(ctx)
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
	emailKey := "admin-email:" + email
	// event distingue el tipo de fallo en el log: "contraseña bien, TOTP mal" indica una contraseña filtrada.
	fail := func(event string) {
		a.accounts.fail(emailKey)
		a.loginFailed(ctx, r, event, email, "admin", true)
		renderStatus(w, r, http.StatusUnauthorized, views.AdminLogin(email, adminFailMsg))
	}
	tooMany := func() {
		Log(ctx).Warn("admin: login bloqueado por límite", "event", "admin_login_limited", "ip", clientIP(r), "email_tag", emailTag(email))
		w.Header().Set("Retry-After", "900")
		renderStatus(w, r, http.StatusTooManyRequests, views.AdminLogin(email, "Demasiados intentos. Espera 15 minutos y vuelve a intentar."))
	}
	if len(email) > 254 || len(pass) > 1024 || len(code) != 6 {
		renderStatus(w, r, http.StatusUnauthorized, views.AdminLogin(email, adminFailMsg))
		return
	}
	if !a.limit.allow("admin-ip:" + limitIP(r)) {
		tooMany()
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
	if a.accounts.blocked(emailKey) && !a.knownDevice(r, adminDeviceCookie, ad.PasswordHash, email) {
		tooMany()
		return
	}
	if ok, err := auth.Verify(pass, hash); err != nil || !ok || ad.ID == 0 {
		fail("admin_login_failed")
		return
	}
	secret, err := auth.Open(a.totpKey, ad.TotpSecret)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	step, ok := auth.VerifyTOTP(secret, code, time.Now())
	if !ok {
		fail("admin_password_ok_totp_failed")
		return
	}
	// Marca el paso como usado: el mismo código no sirve dos veces.
	if n, err := a.q.UseTOTPStep(ctx, db.UseTOTPStepParams{ID: ad.ID, LastTotpStep: step}); err != nil || n == 0 {
		fail("admin_totp_replay")
		return
	}
	if err := a.admin.RenewToken(ctx); err != nil {
		a.fail(w, r, err)
		return
	}
	a.admin.Put(ctx, "admin_id", ad.ID)
	a.admin.Put(ctx, "auth_at", time.Now().Unix())
	a.rememberDevice(w, adminDeviceCookie, "/", ad.PasswordHash, email)
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
	if err := a.audit(ctx, a.q, id, "enlace de recuperación", "para el usuario "+strconv.FormatInt(o.UserID, 10)); err != nil {
		a.fail(w, r, err)
		return
	}
	renderStatus(w, r, http.StatusOK, views.AdminResetLink("Enlace de "+o.Name, link, wa, id))
}

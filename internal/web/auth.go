package web

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/jackc/pgx/v5"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/auth"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/billing"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/web/views"
)

const loginFailMsg = "Email o contraseña incorrectos." // igual para "no existe" y "contraseña mal"

func render(c func(*http.Request) templ.Component) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { renderStatus(w, r, http.StatusOK, c(r)) }
}

func renderStatus(w http.ResponseWriter, r *http.Request, status int, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := c.Render(r.Context(), w); err != nil {
		Log(r.Context()).Error("render", "err", err)
	}
}

// clientIP devuelve la IP de la compradora para los límites de intentos.
// En producción la app escucha en 127.0.0.1 detrás de Caddy, y Caddy solo pone X-Real-IP con la IP real
// cuando la petición viene de Cloudflare (trusted_proxies). Por eso el header solo se cree si la conexión
// llega de loopback; de cualquier otro lado se usa RemoteAddr y el header se ignora (no se puede falsificar).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if real := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); real != nil {
			return real.String()
		}
	}
	return host
}

func (a *app) loginForm(w http.ResponseWriter, r *http.Request) {
	renderStatus(w, r, http.StatusOK, views.Login(StoreFrom(r.Context()).Name, "", ""))
}

func (a *app) loginPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := StoreFrom(ctx)
	email := strings.ToLower(strings.TrimSpace(r.PostFormValue("email")))
	pass := r.PostFormValue("password")
	if len(email) > 254 || len(pass) > 1024 {
		renderStatus(w, r, http.StatusUnauthorized, views.Login(s.Name, "", loginFailMsg))
		return
	}

	tooMany := func() {
		Log(ctx).Warn("login bloqueado por límite", "event", "login_limited", "ip", clientIP(r), "email_tag", emailTag(email))
		w.Header().Set("Retry-After", "900")
		renderStatus(w, r, http.StatusTooManyRequests, views.Login(s.Name, email, "Demasiados intentos. Espera 15 minutos y vuelve a intentar."))
	}
	// La IP cuenta cada intento; si ya está bloqueada no se toca nada más (ni se crean llaves de email).
	if !a.limit.allow("ip:" + limitIP(r)) {
		tooMany()
		return
	}

	u, err := a.q.GetUserForLogin(ctx, db.GetUserForLoginParams{StoreID: s.ID, Email: email})
	hash := auth.DummyHash
	switch {
	case err == nil:
		hash = u.PasswordHash
	case !errors.Is(err, pgx.ErrNoRows):
		Log(ctx).Error("login", "err", err)
		http.Error(w, "Error interno", http.StatusInternalServerError)
		return
	}
	// El email solo cuenta fallos. Bloqueado, solo entra un navegador donde la dueña ya había entrado:
	// así nadie la deja fuera a propósito con 5 intentos cada 15 minutos.
	who := strconv.FormatInt(s.ID, 10) + ":" + email
	emailKey := "email:" + who
	if a.accounts.blocked(emailKey) && !a.knownDevice(r, deviceCookie, u.PasswordHash, who) {
		tooMany()
		return
	}
	ok, err := auth.Verify(pass, hash)
	if err != nil || !ok || u.ID == 0 {
		a.accounts.fail(emailKey)
		a.loginFailed(ctx, r, "login_failed", email, "store_id", s.ID)
		renderStatus(w, r, http.StatusUnauthorized, views.Login(s.Name, email, loginFailMsg))
		return
	}

	if err := a.sessions.RenewToken(ctx); err != nil { // nuevo ID de sesión: evita fijación de sesión
		Log(ctx).Error("renew token", "err", err)
		http.Error(w, "Error interno", http.StatusInternalServerError)
		return
	}
	a.sessions.Put(ctx, "user_id", u.ID)
	a.sessions.Put(ctx, "store_id", s.ID)
	a.sessions.Put(ctx, "auth_at", time.Now().Unix())
	a.rememberDevice(w, deviceCookie, "/panel", u.PasswordHash, who)
	Log(ctx).Info("login", "user_id", u.ID)
	http.Redirect(w, r, "/panel/", http.StatusSeeOther)
}

func (a *app) logout(w http.ResponseWriter, r *http.Request) {
	if err := a.sessions.Destroy(r.Context()); err != nil {
		Log(r.Context()).Error("logout", "err", err)
	}
	http.Redirect(w, r, "/panel/login", http.StatusSeeOther)
}

// requireUser exige sesión y que sea de ESTA tienda: una sesión de luz no abre el panel de ana.
func (a *app) requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		uid, sid := a.sessions.GetInt64(ctx, "user_id"), StoreFrom(ctx).ID
		if uid == 0 || a.sessions.GetInt64(ctx, "store_id") != sid {
			http.Redirect(w, r, "/panel/login", http.StatusSeeOther)
			return
		}
		// Sesiones iniciadas antes de cambiar la contraseña dejan de servir (p. ej. tras recuperarla).
		changed, err := a.q.GetUserPasswordChangedAt(ctx, db.GetUserPasswordChangedAtParams{StoreID: sid, ID: uid})
		if err != nil && !isNoRows(err) {
			a.fail(w, r, err)
			return
		}
		if err != nil || changed.Time.Unix() > a.sessions.GetInt64(ctx, "auth_at") {
			_ = a.sessions.Destroy(ctx)
			http.Redirect(w, r, "/panel/login", http.StatusSeeOther)
			return
		}
		pending, err := a.q.CountPendingOrders(ctx, sid)
		if err != nil { // solo es el número del menú: no bloquea el panel
			Log(ctx).Error("pedidos pendientes", "err", err)
		}
		next.ServeHTTP(w, r.WithContext(views.WithPending(views.WithNotice(ctx, a.billingNotice(ctx, sid)), pending)))
	})
}

// billingNotice: aviso de vencimiento o pausa para el panel; si falla la consulta no se avisa (no bloquea el panel).
func (a *app) billingNotice(ctx context.Context, storeID int64) string {
	sub, err := a.q.GetSubscription(ctx, storeID)
	if err != nil {
		if !isNoRows(err) {
			Log(ctx).Error("suscripción", "err", err)
		}
		return ""
	}
	today, err := a.q.Today(ctx)
	if err != nil {
		Log(ctx).Error("fecha", "err", err)
		return ""
	}
	return billing.Notice(StoreFrom(ctx).Status, dateOf(sub.PaidUntil), today.Time)
}

package web

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/jackc/pgx/v5"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/auth"
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

// clientIP usa RemoteAddr.
// shortcut: detrás de Cloudflare/Caddy todas las IP serán la del proxy; en el sprint 6 leer el header
// del proxy solo cuando la conexión venga de una IP de Cloudflare.
func clientIP(r *http.Request) string {
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return h
	}
	return r.RemoteAddr
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

	ipOK := a.limit.allow("ip:" + clientIP(r))
	emailOK := a.limit.allow("email:" + strconv.FormatInt(s.ID, 10) + ":" + email)
	if !ipOK || !emailOK {
		Log(ctx).Warn("login bloqueado por límite", "ip", clientIP(r))
		renderStatus(w, r, http.StatusTooManyRequests, views.Login(s.Name, email, "Demasiados intentos. Espera 15 minutos y vuelve a intentar."))
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
	ok, err := auth.Verify(pass, hash)
	if err != nil || !ok || u.ID == 0 {
		Log(ctx).Warn("login fallido", "ip", clientIP(r))
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
		if a.sessions.GetInt64(ctx, "user_id") == 0 || a.sessions.GetInt64(ctx, "store_id") != StoreFrom(ctx).ID {
			http.Redirect(w, r, "/panel/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

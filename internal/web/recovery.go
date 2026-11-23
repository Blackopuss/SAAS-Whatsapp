package web

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/auth"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/web/views"
)

const resetTTL = 30 * time.Minute

const resetSentMsg = "Si ese email tiene acceso a esta tienda, te enviamos un enlace. Vence en 30 minutos."

func hashToken(raw string) []byte {
	h := sha256.Sum256([]byte(raw))
	return h[:]
}

// issueReset crea un token de un solo uso; en la base solo queda su hash.
func (a *app) issueReset(ctx context.Context, storeID, userID int64) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	raw := base64.RawURLEncoding.EncodeToString(b)
	return raw, a.q.CreatePasswordReset(ctx, db.CreatePasswordResetParams{
		TokenHash: hashToken(raw), StoreID: storeID, UserID: userID,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(resetTTL), Valid: true},
	})
}

// resetLink lleva el token en la query (?t=) y no en la ruta: el log de peticiones solo guarda la ruta.
func (a *app) resetLink(reqHost, slug, raw string) string {
	host := slug + "." + a.base
	if _, port, err := net.SplitHostPort(reqHost); err == nil {
		host = net.JoinHostPort(host, port)
	}
	scheme := "http"
	if a.secure {
		scheme = "https"
	}
	return scheme + "://" + host + "/panel/recuperar?t=" + url.QueryEscape(raw)
}

func (a *app) forgotForm(w http.ResponseWriter, r *http.Request) {
	renderStatus(w, r, http.StatusOK, views.Forgot(StoreFrom(r.Context()).Name, a.sendReset != nil, "", ""))
}

func (a *app) forgotPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := StoreFrom(ctx)
	if a.sendReset == nil {
		a.forgotForm(w, r)
		return
	}
	email := strings.ToLower(strings.TrimSpace(r.PostFormValue("email")))
	if !a.limit.allow("reset-ip:"+clientIP(r)) || !a.limit.allow("reset-email:"+strconv.FormatInt(s.ID, 10)+":"+email) {
		renderStatus(w, r, http.StatusTooManyRequests, views.Forgot(s.Name, true, "", "Demasiados intentos. Espera 15 minutos y vuelve a intentar."))
		return
	}
	if len(email) <= 254 {
		uid, err := a.q.GetUserByEmail(ctx, db.GetUserByEmailParams{StoreID: s.ID, Email: email})
		switch {
		case err == nil:
			raw, err := a.issueReset(ctx, s.ID, uid)
			if err == nil {
				err = a.sendReset(ctx, email, a.resetLink(r.Host, s.Slug, raw))
			}
			if err != nil {
				a.fail(w, r, err)
				return
			}
			Log(ctx).Info("enlace de recuperación enviado", "user_id", uid)
		case !isNoRows(err):
			a.fail(w, r, err)
			return
		}
	}
	// Misma respuesta exista o no el email: no revela quién tiene cuenta.
	renderStatus(w, r, http.StatusOK, views.Forgot(s.Name, true, resetSentMsg, ""))
}

func (a *app) resetForm(w http.ResponseWriter, r *http.Request) {
	s := StoreFrom(r.Context())
	raw := r.URL.Query().Get("t")
	if _, err := a.q.CheckPasswordReset(r.Context(), db.CheckPasswordResetParams{TokenHash: hashToken(raw), StoreID: s.ID}); err != nil {
		a.resetInvalid(w, r, err)
		return
	}
	renderStatus(w, r, http.StatusOK, views.ResetForm(s.Name, raw, ""))
}

func (a *app) resetPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := StoreFrom(ctx)
	raw := r.URL.Query().Get("t")
	pass := r.PostFormValue("password")
	msg := ""
	switch {
	case len(pass) < 10:
		msg = "La contraseña debe tener al menos 10 caracteres."
	case len(pass) > 1024:
		msg = "La contraseña es demasiado larga."
	case pass != r.PostFormValue("confirm"):
		msg = "Las contraseñas no coinciden."
	}
	if msg != "" {
		renderStatus(w, r, http.StatusUnprocessableEntity, views.ResetForm(s.Name, raw, msg))
		return
	}
	hash, err := auth.Hash(pass)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	var uid int64
	err = a.tx(ctx, func(q db.Querier) error {
		var err error
		if uid, err = q.UsePasswordReset(ctx, db.UsePasswordResetParams{TokenHash: hashToken(raw), StoreID: s.ID}); err != nil {
			return err
		}
		return q.SetUserPassword(ctx, db.SetUserPasswordParams{StoreID: s.ID, ID: uid, PasswordHash: hash})
	})
	if err != nil {
		a.resetInvalid(w, r, err)
		return
	}
	Log(ctx).Info("contraseña cambiada por recuperación", "user_id", uid)
	renderStatus(w, r, http.StatusOK, views.ResetDone(s.Name))
}

func (a *app) resetInvalid(w http.ResponseWriter, r *http.Request, err error) {
	if !isNoRows(err) {
		a.fail(w, r, err)
		return
	}
	renderStatus(w, r, http.StatusGone, views.ResetExpired(StoreFrom(r.Context()).Name))
}

package web

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/auth"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/billing"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/web/views"
)

// audit registra quién hizo qué; va en la misma transacción que el cambio.
func (a *app) audit(ctx context.Context, q db.Querier, storeID int64, action, detail string) error {
	return q.AddAudit(ctx, db.AddAuditParams{
		AdminID: pgtype.Int8{Int64: a.admin.GetInt64(ctx, "admin_id"), Valid: true},
		StoreID: pgtype.Int8{Int64: storeID, Valid: storeID != 0},
		Action:  action, Detail: detail,
	})
}

func dateOf(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	return &d.Time
}

func pgDate(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }

func waLink(phone, text string) string {
	if phone == "" {
		return ""
	}
	return "https://wa.me/" + strings.TrimPrefix(phone, "+") + "?text=" + url.QueryEscape(text)
}

// ---- Alta de tiendas ----

func (a *app) adminStoreNew(w http.ResponseWriter, r *http.Request) {
	renderStatus(w, r, http.StatusOK, views.AdminStoreNew(views.NewStoreForm{}))
}

func (a *app) adminStoreCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	f := views.NewStoreForm{
		Slug:     strings.ToLower(strings.TrimSpace(r.PostFormValue("slug"))),
		Name:     strings.TrimSpace(r.PostFormValue("name")),
		Email:    strings.ToLower(strings.TrimSpace(r.PostFormValue("email"))),
		Whatsapp: r.PostFormValue("whatsapp"),
		Price:    r.PostFormValue("price"),
		Founder:  r.PostFormValue("founder") != "",
	}
	wa, waOK := normalizeWhatsapp(f.Whatsapp)
	price, priceErr := parsePrice(f.Price)
	switch {
	case !ValidSlug(f.Slug):
		f.Error = "El subdominio debe tener de 3 a 30 letras minúsculas, números o guiones, y no puede ser una palabra reservada."
	case f.Name == "" || utf8.RuneCountInString(f.Name) > 80:
		f.Error = "Escribe el nombre de la tienda (máximo 80 letras)."
	case len(f.Email) > 254 || !strings.Contains(f.Email, "@"):
		f.Error = "Escribe un email válido para la dueña."
	case !waOK || len(f.Whatsapp) > 40:
		f.Error = "Revisa el WhatsApp: 10 dígitos si es de México, o empieza con + y la lada."
	case priceErr != nil:
		f.Error = "Escribe el precio mensual en pesos, por ejemplo 149."
	}
	if f.Error != "" {
		renderStatus(w, r, http.StatusUnprocessableEntity, views.AdminStoreNew(f))
		return
	}
	// Contraseña imposible de adivinar: la dueña crea la suya con el enlace de recuperación.
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	hash, err := auth.Hash(base64.RawURLEncoding.EncodeToString(b))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	var storeID, userID int64
	err = a.tx(ctx, func(q db.Querier) error {
		s, err := q.CreateStore(ctx, db.CreateStoreParams{Slug: f.Slug, Name: f.Name})
		if err != nil {
			return err
		}
		storeID = s.ID
		if userID, err = q.CreateUser(ctx, db.CreateUserParams{StoreID: s.ID, Email: f.Email, PasswordHash: hash}); err != nil {
			return err
		}
		if err := q.UpdateStoreWhatsapp(ctx, db.UpdateStoreWhatsappParams{ID: s.ID, Whatsapp: wa}); err != nil {
			return err
		}
		if err := q.UpdateSubscriptionPlan(ctx, db.UpdateSubscriptionPlanParams{StoreID: s.ID, IsFounder: f.Founder, PriceCents: price}); err != nil {
			return err
		}
		return a.audit(ctx, q, s.ID, "alta de tienda", f.Slug+" · "+f.Email)
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		f.Error = "Ese subdominio ya está ocupado. Elige otro."
		renderStatus(w, r, http.StatusUnprocessableEntity, views.AdminStoreNew(f))
		return
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	raw, err := a.issueReset(ctx, storeID, userID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	link := a.resetLink(r.Host, f.Slug, raw)
	msg := "¡Hola! Tu tienda " + f.Name + " ya está lista. Entra a este enlace para crear tu contraseña (vence en 30 minutos): " + link
	renderStatus(w, r, http.StatusOK, views.AdminResetLink("Tienda creada: "+f.Name, link, waLink(wa, msg), storeID))
}

// ---- Detalle de tienda ----

func (a *app) adminStore(w http.ResponseWriter, r *http.Request) {
	a.renderAdminStore(w, r, http.StatusOK, "", "")
}

func (a *app) renderAdminStore(w http.ResponseWriter, r *http.Request, status int, errMsg, okMsg string) {
	ctx := r.Context()
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	s, err := a.q.GetStoreAdmin(ctx, id)
	if err != nil {
		a.notFoundOr500(w, r, err)
		return
	}
	pays, err := a.q.ListPayments(ctx, id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	today, err := a.q.Today(ctx)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if okMsg == "" {
		okMsg = adminOK[r.URL.Query().Get("ok")]
	}
	v := views.AdminStoreView{
		Store: s, Payments: pays, Today: today.Time.Format("2006-01-02"), ShopURL: a.shopURL(r, s.Slug),
		Error: errMsg, OK: okMsg, Notice: billing.Notice(s.Status, dateOf(s.PaidUntil), today.Time),
	}
	if s.PaidUntil.Valid {
		v.PaidUntil = billing.Date(s.PaidUntil.Time)
	}
	if v.Products, err = a.q.ListPanelProducts(ctx, id); err != nil {
		a.fail(w, r, err)
		return
	}
	renderStatus(w, r, status, views.AdminStore(v))
}

var adminOK = map[string]string{
	"pago":       "Pago registrado.",
	"pausada":    "Tienda pausada.",
	"reactivada": "Tienda reactivada.",
	"plan":       "Plan guardado.",
	"precio":     "Precio guardado.",
}

func (a *app) adminPayment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	amount, err := parsePrice(r.PostFormValue("amount"))
	months, mErr := strconv.Atoi(r.PostFormValue("months"))
	paidOn, dErr := time.Parse("2006-01-02", r.PostFormValue("paid_on"))
	note := strings.TrimSpace(r.PostFormValue("note"))
	today, tErr := a.q.Today(ctx)
	if tErr != nil {
		a.fail(w, r, tErr)
		return
	}
	switch {
	case err != nil || amount <= 0:
		a.renderAdminStore(w, r, http.StatusUnprocessableEntity, "Escribe el monto pagado en pesos, por ejemplo 149.", "")
		return
	case mErr != nil || months < 1 || months > 12:
		a.renderAdminStore(w, r, http.StatusUnprocessableEntity, "Elige de 1 a 12 meses.", "")
		return
	case dErr != nil || paidOn.After(today.Time) || paidOn.Before(today.Time.AddDate(-1, 0, 0)):
		a.renderAdminStore(w, r, http.StatusUnprocessableEntity, "La fecha del pago no puede ser futura ni de hace más de un año.", "")
		return
	case utf8.RuneCountInString(note) > 200:
		a.renderAdminStore(w, r, http.StatusUnprocessableEntity, "La nota puede tener hasta 200 letras.", "")
		return
	}
	var slug string
	err = a.tx(ctx, func(q db.Querier) error {
		sub, err := q.LockSubscription(ctx, id) // dos pagos al mismo tiempo no pisan la fecha
		if err != nil {
			return err
		}
		until, streak, onTime := billing.Apply(dateOf(sub.PaidUntil), sub.StreakMonths, paidOn, months)
		if err := q.ApplyPayment(ctx, db.ApplyPaymentParams{StoreID: id, PaidUntil: pgDate(until), StreakMonths: streak}); err != nil {
			return err
		}
		adminID := pgtype.Int8{Int64: a.admin.GetInt64(ctx, "admin_id"), Valid: true}
		if err := q.CreatePayment(ctx, db.CreatePaymentParams{
			StoreID: id, AmountCents: amount, Months: int32(months), PaidOn: pgDate(paidOn), Note: note, // #nosec G109 G115 -- validado 1..12 arriba
			OnTime: onTime, PrevPaidUntil: sub.PaidUntil, NewPaidUntil: pgDate(until), AdminID: adminID,
		}); err != nil {
			return err
		}
		if err := a.audit(ctx, q, id, "pago registrado", views.Money(amount)+" por "+strconv.Itoa(months)+" mes(es); vence el "+billing.Date(until)); err != nil {
			return err
		}
		s, err := q.GetStoreAdmin(ctx, id)
		if err != nil {
			return err
		}
		slug = s.Slug
		// Si estaba pausada por atraso y ya quedó al corriente, se reabre sola.
		if s.Status == "suspended" && !until.Before(today.Time) {
			if _, err := q.SetStoreStatus(ctx, db.SetStoreStatusParams{ID: id, Status: "active"}); err != nil {
				return err
			}
			return a.audit(ctx, q, id, "reactivada", "al registrar pago")
		}
		return nil
	})
	if err != nil {
		a.notFoundOr500(w, r, err)
		return
	}
	a.tenants.forget(slug)
	back(w, r, "/tiendas/"+strconv.FormatInt(id, 10)+"?ok=pago")
}

func (a *app) adminSetStatus(status, action, ok string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		id, okID := pathID(w, r)
		if !okID {
			return
		}
		var slug string
		err := a.tx(ctx, func(q db.Querier) error {
			s, err := q.GetStoreAdmin(ctx, id)
			if err != nil {
				return err
			}
			slug = s.Slug
			n, err := q.SetStoreStatus(ctx, db.SetStoreStatusParams{ID: id, Status: status})
			if err != nil || n == 0 {
				return err
			}
			return a.audit(ctx, q, id, action, strings.TrimSpace(r.PostFormValue("reason")))
		})
		if err != nil {
			a.notFoundOr500(w, r, err)
			return
		}
		a.tenants.forget(slug)
		back(w, r, "/tiendas/"+strconv.FormatInt(id, 10)+"?ok="+ok)
	}
}

func (a *app) adminPlan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	price, err := parsePrice(r.PostFormValue("price"))
	if err != nil {
		a.renderAdminStore(w, r, http.StatusUnprocessableEntity, "Escribe el precio mensual en pesos, por ejemplo 149.", "")
		return
	}
	founder := r.PostFormValue("founder") != ""
	err = a.tx(ctx, func(q db.Querier) error {
		if _, err := q.GetStoreAdmin(ctx, id); err != nil {
			return err
		}
		if err := q.UpdateSubscriptionPlan(ctx, db.UpdateSubscriptionPlanParams{StoreID: id, IsFounder: founder, PriceCents: price}); err != nil {
			return err
		}
		detail := views.Money(price) + " al mes"
		if founder {
			detail += ", fundadora"
		}
		return a.audit(ctx, q, id, "plan cambiado", detail)
	})
	if err != nil {
		a.notFoundOr500(w, r, err)
		return
	}
	back(w, r, "/tiendas/"+strconv.FormatInt(id, 10)+"?ok=plan")
}

// adminProductPrice: el precio normal de un producto ya creado solo lo cambia el superadmin, y queda en la bitácora.
func (a *app) adminProductPrice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	pid, err := strconv.ParseInt(r.PathValue("pid"), 10, 64)
	if err != nil || pid <= 0 {
		http.NotFound(w, r)
		return
	}
	price, err := parsePrice(r.PostFormValue("price"))
	if err != nil {
		a.renderAdminStore(w, r, http.StatusUnprocessableEntity, "Precio: "+err.Error()+".", "")
		return
	}
	err = a.tx(ctx, func(q db.Querier) error {
		if _, err := q.GetStoreAdmin(ctx, id); err != nil {
			return err
		}
		p, err := q.SetProductPrice(ctx, db.SetProductPriceParams{StoreID: id, ID: pid, PriceCents: price})
		if err != nil {
			return err
		}
		if p.OldPriceCents == price {
			return nil
		}
		return a.audit(ctx, q, id, "precio cambiado", p.Name+": "+views.Money(p.OldPriceCents)+" → "+views.Money(price))
	})
	if err != nil {
		a.notFoundOr500(w, r, err)
		return
	}
	back(w, r, "/tiendas/"+strconv.FormatInt(id, 10)+"?ok=precio#precios")
}

func (a *app) adminAudit(w http.ResponseWriter, r *http.Request) {
	rows, err := a.q.ListAudit(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	renderStatus(w, r, http.StatusOK, views.AdminAudit(rows))
}

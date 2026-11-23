// Package billing: cobro manual, racha de lealtad y corte por atraso.
package billing

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
)

const (
	GraceDays  = 5 // al sexto día de atraso la tienda se pausa
	NoticeDays = 3 // aviso en el panel desde 3 días antes de vencer
)

// Apply calcula el efecto de un pago. Pagar antes o el mismo día del vencimiento mantiene la racha;
// pagar tarde la reinicia y el nuevo periodo cuenta desde el día del pago (no se cobran días en pausa).
func Apply(prev *time.Time, streak int32, paidOn time.Time, months int) (newUntil time.Time, newStreak int32, onTime bool) {
	onTime = prev == nil || !paidOn.After(*prev)
	start := paidOn
	if prev != nil && prev.After(paidOn) {
		start = *prev
	}
	m := int32(months) // #nosec G115 -- validado 1..12
	newStreak = m
	if onTime && prev != nil {
		newStreak = streak + m
	}
	return start.AddDate(0, months, 0), newStreak, onTime
}

// SuspendsOn es el primer día en que la tienda queda pausada si no paga.
func SuspendsOn(paidUntil time.Time) time.Time { return paidUntil.AddDate(0, 0, GraceDays+1) }

// Notice es el aviso para el panel de la vendedora; vacío si no hay nada que avisar.
func Notice(status string, paidUntil *time.Time, today time.Time) string {
	if status != "active" { // suspended (por atraso) o paused (a mano)
		return "Tu tienda está en pausa: las compradoras no pueden verla. Escríbenos por WhatsApp para reactivarla."
	}
	if paidUntil == nil {
		return ""
	}
	days := int(paidUntil.Sub(today).Hours() / 24)
	switch {
	case days < 0:
		return "Tu pago venció el " + Date(*paidUntil) + ". Si no lo recibimos, tu tienda se pausará el " + Date(SuspendsOn(*paidUntil)) + "."
	case days == 0:
		return "Tu plan vence hoy. Paga para que tu tienda siga abierta."
	case days <= NoticeDays:
		return "Tu plan vence el " + Date(*paidUntil) + " (en " + strconv.Itoa(days) + plural(days, " día", " días") + ")."
	}
	return ""
}

var months = [...]string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}

// Date: "25 de noviembre de 2026".
func Date(t time.Time) string {
	return strconv.Itoa(t.Day()) + " de " + months[t.Month()-1] + " de " + strconv.Itoa(t.Year())
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// Run pausa cada hora las tiendas con más de GraceDays de atraso y vence los pedidos pendientes de más de 7 días. Es idempotente: correrlo de más no daña.
// shortcut: goroutine en el mismo proceso en vez de un job de River; con una sola instancia basta.
// Pasar a River si se corre más de una instancia o se agregan avisos por WhatsApp/email.
func Run(ctx context.Context, q db.Querier, onSuspend func(slug string)) {
	tick := time.NewTicker(time.Hour)
	defer tick.Stop()
	for {
		suspendOverdue(ctx, q, onSuspend)
		if n, err := q.ExpireOrders(ctx); err != nil {
			slog.Error("vencer pedidos", "err", err)
		} else if n > 0 {
			slog.Info("pedidos sin respuesta en 7 días", "n", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func suspendOverdue(ctx context.Context, q db.Querier, onSuspend func(string)) {
	rows, err := q.SuspendOverdue(ctx)
	if err != nil {
		slog.Error("corte por atraso", "err", err)
		return
	}
	for _, s := range rows {
		slog.Warn("tienda pausada por atraso", "store_id", s.ID)
		if err := q.AddAudit(ctx, db.AddAuditParams{StoreID: pgtype.Int8{Int64: s.ID, Valid: true}, Action: "pausa automática", Detail: "más de " + strconv.Itoa(GraceDays) + " días de atraso"}); err != nil {
			slog.Error("bitácora", "err", err)
		}
		onSuspend(s.Slug)
	}
}

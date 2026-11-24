package web

import (
	"net/http"
	"time"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/web/views"
)

// home: Inicio del panel. Dice a la vendedora qué le falta para dejar lista su tienda y qué hay que atender.
func (a *app) home(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s, err := a.q.GetStoreBySlug(ctx, StoreFrom(ctx).Slug) // sin caché: el paso recién hecho ya sale como hecho
	if err != nil {
		a.fail(w, r, err)
		return
	}
	ps, err := a.q.ListPanelProducts(ctx, s.ID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	cats, err := a.q.ListCategories(ctx, s.ID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	p := periodFor("mes", time.Now())
	totals, err := a.q.SalesTotals(ctx, db.SalesTotalsParams{StoreID: s.ID, Since: ts(p.since), Until: ts(p.till)})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	v := views.HomeData{
		ShopURL: a.shopURL(r, s.Slug), HasLogo: s.LogoKey != "", HasWhatsapp: s.Whatsapp != "", HasPayment: s.PaymentInfo != "",
		Categories: len(cats), Products: len(ps),
	}
	for _, p := range ps {
		if p.ImageKey != "" {
			v.WithPhoto++
		}
		switch {
		case p.SoldOut || (p.Stock >= 0 && p.Stock < p.Step):
			v.SoldOut++
		case p.Stock >= 0 && p.Stock <= p.Step*3:
			v.Low++
		}
	}
	for _, t := range totals {
		if t.Status == orderSold {
			v.Sold += t.Orders
			v.Revenue += t.Cents
		}
	}
	renderStatus(w, r, http.StatusOK, views.PanelHome(s.Name, v))
}

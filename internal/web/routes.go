// Package web arma el router HTTP y sus middlewares.
package web

import (
	"context"
	"net/http"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/web/views"
)

type Pinger interface{ Ping(context.Context) error }

func Routes(db Pinger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(r.Context()); err != nil {
			Log(r.Context()).Error("healthz", "err", err)
			http.Error(w, "db no disponible", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if err := views.Hello().Render(r.Context(), w); err != nil {
			Log(r.Context()).Error("render", "err", err)
		}
	})
	return chain(mux)
}

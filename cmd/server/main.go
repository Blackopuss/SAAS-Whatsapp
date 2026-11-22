package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexedwards/scs/pgxstore"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/billing"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/config"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/storage"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/web"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("arranque", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		return err
	}

	var store storage.Storage = storage.Disk{Dir: "uploads"}
	mediaURL, mediaDir := "/media/", "uploads"
	if cfg.HasR2() {
		r2, err := storage.NewR2(cfg.R2AccountID, cfg.R2AccessKey, cfg.R2SecretKey, cfg.R2Bucket)
		if err != nil {
			return err
		}
		store, mediaURL, mediaDir = r2, cfg.R2PublicURL+"/", ""
	}
	slog.Info("fotos", "destino", mediaURL)

	// shortcut: sin proveedor de email/WhatsApp; en desarrollo el enlace sale en el log y en producción
	// la vendedora lo pide a soporte (superadmin). Conectar Cloudflare Email cuando haya dominio.
	var sendReset func(context.Context, string, string) error
	if cfg.Env == "dev" {
		sendReset = func(_ context.Context, to, link string) error {
			slog.Info("enlace de recuperación (solo desarrollo)", "para", to, "enlace", link)
			return nil
		}
	}

	go billing.Run(ctx, db.New(pool), func(string) {}) // el caché de tiendas vence solo en 60 s

	srv := &http.Server{
		Addr: cfg.Addr,
		Handler: http.MaxBytesHandler(web.Routes(web.Deps{
			BaseDomain: cfg.BaseDomain,
			Secure:     cfg.Env == "prod",
			DB:         pool,
			Q:          db.New(pool),
			Tx:         func(ctx context.Context, fn func(db.Querier) error) error { return db.InTx(ctx, pool, fn) },
			Sessions:   pgxstore.New(pool),
			Storage:    store,
			MediaURL:   mediaURL,
			MediaDir:   mediaDir,
			TOTPKey:    cfg.TOTPKey,
			SendReset:  sendReset,
		}), 12<<20), // 10 MB por foto + margen del formulario
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Info("escuchando", "addr", cfg.Addr, "env", cfg.Env)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

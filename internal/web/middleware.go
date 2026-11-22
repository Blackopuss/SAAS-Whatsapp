package web

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"net/http"
	"time"

	"github.com/a-h/templ"
)

type ctxKey int

const (
	loggerKey ctxKey = iota
	storeKey
)

// reqLog es un puntero compartido para que el tenant agregue store_id y la línea final del logger lo incluya.
type reqLog struct{ l *slog.Logger }

// Log devuelve el logger de la petición, con request_id y, dentro de una tienda, store_id.
func Log(ctx context.Context) *slog.Logger {
	if rl, ok := ctx.Value(loggerKey).(*reqLog); ok {
		return rl.l
	}
	return slog.Default()
}

func addLogAttrs(ctx context.Context, args ...any) {
	if rl, ok := ctx.Value(loggerKey).(*reqLog); ok {
		rl.l = rl.l.With(args...)
	}
}

func randToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b) // crypto/rand.Read nunca falla desde Go 1.24
	return base64.RawURLEncoding.EncodeToString(b)
}

func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				Log(r.Context()).Error("panic", "err", v)
				http.Error(w, "Error interno", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// requestID genera siempre su propio ID: no confiamos en el header que mande el cliente.
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := randToken(12)
		w.Header().Set("X-Request-Id", id)
		rl := &reqLog{slog.Default().With("request_id", id)}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), loggerKey, rl)))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) { s.status = code; s.ResponseWriter.WriteHeader(code) }
func (s *statusWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }

func logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		Log(r.Context()).Info("http", "method", r.Method, "path", r.URL.Path, "status", sw.status, "dur_ms", time.Since(start).Milliseconds())
	})
}

// securityHeaders pone los headers fijos y un nonce de CSP por petición, que templ inserta en <script>/<style>.
// frame-ancestors 'none' aplica a todo por ahora; en el sprint 4 se abre solo para la vista previa del panel.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce := randToken(18)
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'nonce-"+nonce+"'; style-src 'self' 'nonce-"+nonce+"'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self' https://wa.me; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=(), interest-cohort=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		next.ServeHTTP(w, r.WithContext(templ.WithNonce(r.Context(), nonce)))
	})
}

// chain aplica los middlewares en el orden dado (el primero es el más externo).
func chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

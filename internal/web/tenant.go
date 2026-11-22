package web

import (
	"context"
	"errors"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
)

var (
	slugRe   = regexp.MustCompile(`^[a-z0-9-]{3,30}$`)
	reserved = map[string]bool{"www": true, "panel": true, "admin": true, "api": true, "mail": true, "static": true}
)

// ValidSlug aplica las reglas de slug del plan: a-z0-9-, 3 a 30, sin subdominios reservados.
func ValidSlug(s string) bool { return slugRe.MatchString(s) && !reserved[s] }

// slugFromHost devuelve el slug de "ana.dominio" o "" si el host es el dominio raíz o ajeno.
func slugFromHost(host, base string) (slug string, isRoot bool) {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == base {
		return "", true
	}
	slug, ok := strings.CutSuffix(host, "."+base)
	if !ok || strings.Contains(slug, ".") {
		return "", false
	}
	return slug, false
}

type cachedStore struct {
	store db.Store
	exp   time.Time
}

// tenants cachea solo tiendas encontradas (60 s); los slugs inexistentes no se cachean para que
// alguien pidiendo subdominios al azar no llene la memoria.
type tenants struct {
	mu     sync.RWMutex
	m      map[string]cachedStore
	lookup func(context.Context, string) (db.Store, error)
	base   string
}

func (t *tenants) get(ctx context.Context, slug string) (db.Store, error) {
	t.mu.RLock()
	c, ok := t.m[slug]
	t.mu.RUnlock()
	if ok && time.Now().Before(c.exp) {
		return c.store, nil
	}
	s, err := t.lookup(ctx, slug)
	if err != nil {
		return db.Store{}, err
	}
	t.mu.Lock()
	t.m[slug] = cachedStore{s, time.Now().Add(60 * time.Second)}
	t.mu.Unlock()
	return s, nil
}

func (t *tenants) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slug, _ := slugFromHost(r.Host, t.base)
		if !ValidSlug(slug) {
			http.NotFound(w, r)
			return
		}
		s, err := t.get(r.Context(), slug)
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			Log(r.Context()).Error("tenant", "err", err)
			http.Error(w, "Error interno", http.StatusInternalServerError)
			return
		}
		addLogAttrs(r.Context(), "store_id", s.ID)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), storeKey, s)))
	})
}

// StoreFrom devuelve la tienda del contexto; solo llamar dentro de rutas de tienda.
func StoreFrom(ctx context.Context) db.Store {
	return ctx.Value(storeKey).(db.Store)
}

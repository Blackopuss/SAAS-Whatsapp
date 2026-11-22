package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeDB struct{ err error }

func (f fakeDB) Ping(context.Context) error { return f.err }

func TestSecurityHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	Routes(fakeDB{}).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	h := rec.Header()
	csp := h.Get("Content-Security-Policy")
	for _, want := range []string{"'nonce-", "frame-ancestors 'none'", "object-src 'none'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP sin %q: %s", want, csp)
		}
	}
	for k, v := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	} {
		if h.Get(k) != v {
			t.Errorf("%s = %q, quiero %q", k, h.Get(k), v)
		}
	}
	if h.Get("Permissions-Policy") == "" || h.Get("X-Request-Id") == "" {
		t.Error("falta Permissions-Policy o X-Request-Id")
	}
}

func TestNoncePerRequest(t *testing.T) {
	h := Routes(fakeDB{})
	a, b := httptest.NewRecorder(), httptest.NewRecorder()
	h.ServeHTTP(a, httptest.NewRequest("GET", "/", nil))
	h.ServeHTTP(b, httptest.NewRequest("GET", "/", nil))
	if a.Header().Get("Content-Security-Policy") == b.Header().Get("Content-Security-Policy") {
		t.Fatal("el nonce se repite entre peticiones")
	}
}

func TestRecoverer(t *testing.T) {
	h := chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("status = %d body = %q", rec.Code, rec.Body.String())
	}
}

func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	Routes(fakeDB{err: errors.New("down")}).ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
}

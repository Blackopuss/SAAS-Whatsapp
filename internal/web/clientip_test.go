package web

import (
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	cases := []struct{ remote, header, want string }{
		{"127.0.0.1:5000", "203.0.113.7", "203.0.113.7"},     // detrás de Caddy
		{"[::1]:5000", "2001:db8::1", "2001:db8::1"},         // Caddy por IPv6 local
		{"127.0.0.1:5000", "basura", "127.0.0.1"},            // header inválido
		{"198.51.100.9:5000", "203.0.113.7", "198.51.100.9"}, // directo: el header no cuenta
		{"198.51.100.9:5000", "", "198.51.100.9"},
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = c.remote
		if c.header != "" {
			r.Header.Set("X-Real-IP", c.header)
		}
		if got := clientIP(r); got != c.want {
			t.Errorf("remote %s header %q: %s, quiero %s", c.remote, c.header, got, c.want)
		}
	}
}

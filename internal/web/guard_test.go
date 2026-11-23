package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	now := time.Unix(1000, 0)
	l := newLimiter(2, time.Minute)
	l.now = func() time.Time { return now }

	// fail cuenta sin preguntar; blocked pregunta sin contar.
	if l.blocked("e") {
		t.Fatal("bloqueada sin intentos")
	}
	l.fail("e")
	l.fail("e")
	if !l.blocked("e") {
		t.Fatal("dos fallos deben bloquear")
	}
	if l.hits["e"].count != 2 {
		t.Fatalf("blocked no debe contar: %d intentos", l.hits["e"].count)
	}
	now = now.Add(time.Minute + time.Second)
	if l.blocked("e") {
		t.Fatal("la ventana venció")
	}

	// El mapa no pasa de maxKeys aunque lleguen llaves nuevas sin parar.
	for i := range maxKeys + 500 {
		l.allow("k" + strconv.Itoa(i))
	}
	if len(l.hits) > maxKeys {
		t.Fatalf("el mapa creció a %d llaves", len(l.hits))
	}
}

// Lleno de llaves vivas, el limitador de cuentas no suelta el bloqueo de la víctima y falla cerrado.
func TestStrictLimiterKeepsLockouts(t *testing.T) {
	l := newStrictLimiter(2, time.Minute)
	l.fail("victima")
	l.fail("victima")
	for i := range maxKeys + 500 {
		l.fail("k" + strconv.Itoa(i))
	}
	if !l.blocked("victima") {
		t.Fatal("la inundación soltó el bloqueo de la víctima")
	}
	if !l.blocked("nueva") {
		t.Fatal("lleno, una cuenta nueva debe contar como bloqueada")
	}
	if len(l.hits) > maxKeys {
		t.Fatalf("el mapa creció a %d", len(l.hits))
	}
}

func TestLimitIP(t *testing.T) {
	for _, c := range []struct{ header, want string }{
		{"203.0.113.7", "203.0.113.7"},
		{"2001:db8:1:2:aaaa::1", "2001:db8:1:2::/64"},
		{"2001:db8:1:2:bbbb::9", "2001:db8:1:2::/64"}, // mismo /64: misma llave
	} {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = "127.0.0.1:5000"
		r.Header.Set("X-Real-IP", c.header)
		if got := limitIP(r); got != c.want {
			t.Errorf("%s: %s, quiero %s", c.header, got, c.want)
		}
	}
}

func TestPublicStock(t *testing.T) {
	for _, c := range []struct{ avail, step, want int32 }{
		{-1, 1000, -1}, {0, 1000, 0}, {3000, 1000, 3000}, {500_000, 1000, 99_000}, {250, 250, 250},
	} {
		if got := publicStock(c.avail, c.step); got != c.want {
			t.Errorf("publicStock(%d, %d) = %d, quiero %d", c.avail, c.step, got, c.want)
		}
	}
}

// Quien conoce el email de la dueña puede bloquearlo con 5 fallos, pero ella sigue entrando
// desde el navegador donde ya había entrado.
func TestLockoutSparesKnownDevice(t *testing.T) {
	h := testRoutes(nil)
	post := func(ip, pass string, c *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "http://ana.tiendas.test/panel/login", strings.NewReader(url.Values{"email": {"ana@x.mx"}, "password": {pass}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.RemoteAddr = ip + ":5555"
		if c != nil {
			r.AddCookie(c)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec
	}
	ok := post("198.51.100.1", "contraseña-segura", nil)
	var device *http.Cookie
	for _, c := range ok.Result().Cookies() {
		if c.Name == deviceCookie {
			device = c
		}
	}
	if ok.Code != http.StatusSeeOther || device == nil || !device.HttpOnly {
		t.Fatalf("login: status %d, cookie de dispositivo %+v", ok.Code, device)
	}
	for range 5 {
		post("198.51.100.2", "mala", nil)
	}
	if rec := post("198.51.100.3", "contraseña-segura", nil); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("email bloqueado sin dispositivo conocido: status %d", rec.Code)
	}
	if rec := post("198.51.100.3", "contraseña-segura", device); rec.Code != http.StatusSeeOther {
		t.Fatalf("dueña desde su navegador: status %d", rec.Code)
	}
	forged := &http.Cookie{Name: deviceCookie, Value: "falsa"}
	if rec := post("198.51.100.4", "contraseña-segura", forged); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("cookie falsa: status %d", rec.Code)
	}
}

func TestNoDirectoryListing(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "stores", "1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stores", "1", "foto.webp"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := http.FileServer(noDirs{http.Dir(dir)})
	for path, want := range map[string]int{"/stores/1/": 404, "/stores/": 404, "/stores/1/foto.webp": 200} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != want {
			t.Errorf("%s: status %d, quiero %d", path, rec.Code, want)
		}
	}
}

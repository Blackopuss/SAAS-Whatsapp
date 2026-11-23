package web

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net"
	"net/http"
	"time"
)

// limitIP es la llave de IP para los límites de intentos. En IPv6 cuenta el bloque /64:
// una sola conexión casera tiene millones de direcciones y rotarlas no debe dar intentos nuevos.
func limitIP(r *http.Request) string {
	ip := clientIP(r)
	if p := net.ParseIP(ip); p != nil && p.To4() == nil {
		return p.Mask(net.CIDRMask(64, 128)).String() + "/64"
	}
	return ip
}

// Dispositivo conocido: tras entrar bien, el navegador guarda una marca firmada con el hash de la
// contraseña de la cuenta. Con ella la dueña puede entrar aunque alguien más haya bloqueado su email
// a propósito con intentos fallidos (el límite por IP sigue aplicando). Cambiar la contraseña la invalida.
const (
	deviceCookie      = "tiendas_disp"
	adminDeviceCookie = "tiendas_admin_disp"
)

// La llave junta TOTP_KEY (fuera de la base) y el hash: quien solo tenga la base o un respaldo no la falsifica.
func (a *app) deviceMAC(passwordHash, who string) string {
	m := hmac.New(sha256.New, append(append([]byte{}, a.totpKey...), passwordHash...))
	m.Write([]byte("dispositivo:" + who))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func (a *app) rememberDevice(w http.ResponseWriter, name, path, passwordHash, who string) {
	http.SetCookie(w, &http.Cookie{ // #nosec G124 -- HttpOnly y Lax siempre; Secure = a.secure (true en prod, false solo en dev por http)
		Name: name, Value: a.deviceMAC(passwordHash, who), Path: path, MaxAge: 365 * 24 * 3600,
		HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode,
	})
}

func (a *app) knownDevice(r *http.Request, name, passwordHash, who string) bool {
	if passwordHash == "" {
		return false
	}
	c, err := r.Cookie(name)
	return err == nil && hmac.Equal([]byte(c.Value), []byte(a.deviceMAC(passwordHash, who)))
}

// emailTag identifica la cuenta atacada en el log sin guardar el email.
func emailTag(email string) string {
	h := sha256.Sum256([]byte(email))
	return hex.EncodeToString(h[:6])
}

// loginFailed registra el intento fallido con su tipo y, si en 10 minutos van más de 30 en todo el
// servidor, deja una alerta (nivel ERROR, evento login_alert) para que la recoja el monitoreo de logs.
func (a *app) loginFailed(ctx context.Context, r *http.Request, event, email string, attrs ...any) {
	Log(ctx).Warn("login fallido", append([]any{"event", event, "ip", clientIP(r), "email_tag", emailTag(email)}, attrs...)...)
	if a.failAlarm.count("login") == failAlarmMax+1 {
		Log(ctx).Error("alerta: muchos logins fallidos", "event", "login_alert", "window", failAlarmWindow.String())
	}
}

const (
	failAlarmMax    = 30
	failAlarmWindow = 10 * time.Minute
)

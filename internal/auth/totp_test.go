package auth

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestTOTPRFC6238(t *testing.T) {
	secret := []byte("12345678901234567890")
	// Vectores SHA1 del RFC 6238 (los últimos 6 de los 8 dígitos).
	for unix, want := range map[int64]string{59: "287082", 1111111109: "081804", 1234567890: "005924", 2000000000: "279037"} {
		if got := TOTPCode(secret, unix/30); got != want {
			t.Errorf("t=%d: %s, quiero %s", unix, got, want)
		}
	}
	now := time.Unix(1111111109, 0)
	if _, ok := VerifyTOTP(secret, "081804", now); !ok {
		t.Fatal("código actual rechazado")
	}
	if _, ok := VerifyTOTP(secret, TOTPCode(secret, now.Unix()/30-1), now); !ok {
		t.Fatal("código del paso anterior rechazado")
	}
	if _, ok := VerifyTOTP(secret, TOTPCode(secret, now.Unix()/30-3), now); ok {
		t.Fatal("aceptó un código viejo")
	}
	if !strings.HasPrefix(TOTPURI(secret, "Tiendas", "yo@x.mx"), "otpauth://totp/Tiendas:yo@x.mx?") {
		t.Fatal(TOTPURI(secret, "Tiendas", "yo@x.mx"))
	}
}

func TestSealOpen(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	sealed, err := Seal(key, []byte("secreto"))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Open(key, sealed); err != nil || string(got) != "secreto" {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := Open(bytes.Repeat([]byte{8}, 32), sealed); err == nil {
		t.Fatal("abrió con otra llave")
	}
	sealed[len(sealed)-1] ^= 1
	if _, err := Open(key, sealed); err == nil {
		t.Fatal("aceptó un dato alterado")
	}
}

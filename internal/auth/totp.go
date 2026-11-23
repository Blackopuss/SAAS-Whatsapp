package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" // #nosec G505 -- RFC 6238 usa HMAC-SHA1; es lo que leen las apps de autenticación
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"time"
)

const totpPeriod = 30

// TOTPCode calcula el código de 6 dígitos para un paso de 30 s (RFC 6238).
func TOTPCode(secret []byte, step int64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step)) // #nosec G115 -- el paso siempre es positivo
	mac := hmac.New(sha1.New, secret)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	n := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", n%1_000_000)
}

// VerifyTOTP acepta el paso actual y uno antes o después (reloj del celular desfasado).
// Devuelve el paso que coincidió, para que quien llama impida reusarlo.
func VerifyTOTP(secret []byte, code string, now time.Time) (int64, bool) {
	step := now.Unix() / totpPeriod
	for _, s := range []int64{step, step - 1, step + 1} {
		if hmac.Equal([]byte(TOTPCode(secret, s)), []byte(code)) {
			return s, true
		}
	}
	return 0, false
}

func NewTOTPSecret() []byte {
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	return b
}

// TOTPURI es lo que se convierte en QR para Google Authenticator, 1Password, etc.
func TOTPURI(secret []byte, issuer, account string) string {
	s := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret)
	return "otpauth://totp/" + url.PathEscape(issuer+":"+account) + "?" + url.Values{"secret": {s}, "issuer": {issuer}}.Encode()
}

// Seal y Open cifran el secreto TOTP con AES-256-GCM: si se filtra la base, el segundo factor sigue a salvo.
func Seal(key, plain []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, nil), nil
}

func Open(key, sealed []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, errors.New("dato cifrado inválido")
	}
	return gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], nil)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, errors.New("TOTP_KEY debe medir 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

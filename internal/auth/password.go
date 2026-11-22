// Package auth hashea y verifica contraseñas con argon2id (formato PHC).
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Parámetros mínimos recomendados por OWASP para argon2id (19 MiB, 2 pasadas, 1 hilo).
const (
	memKiB  = 19 * 1024
	passes  = 2
	threads = 1
	keyLen  = 32
)

var b64 = base64.RawStdEncoding

func Hash(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, passes, memKiB, threads, keyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, memKiB, passes, threads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// Verify lee los parámetros del hash guardado, así se pueden subir en el futuro sin romper hashes viejos.
func Verify(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("hash con formato inválido")
	}
	var v int
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[2], "v=%d", &v); err != nil || v != argon2.Version {
		return false, errors.New("versión de argon2 no soportada")
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false, errors.New("parámetros de argon2 inválidos")
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(password), salt, t, m, p, uint32(len(want))) // #nosec G115 -- len de un hash de 32 bytes
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// DummyHash se usa cuando el email no existe, para que la respuesta tarde lo mismo que con una contraseña mal.
var DummyHash = func() string {
	h, err := Hash("dummy-password-for-timing")
	if err != nil {
		panic(err)
	}
	return h
}()

package auth

import (
	"strings"
	"testing"
)

func TestHashVerify(t *testing.T) {
	h, err := Hash("secreto123")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("formato inesperado: %s", h)
	}
	if ok, err := Verify("secreto123", h); !ok || err != nil {
		t.Fatalf("debería verificar: ok=%v err=%v", ok, err)
	}
	if ok, _ := Verify("otra", h); ok {
		t.Fatal("contraseña incorrecta verificó")
	}
	h2, _ := Hash("secreto123")
	if h == h2 {
		t.Fatal("la sal no es aleatoria")
	}
	if _, err := Verify("x", "$bcrypt$basura"); err == nil {
		t.Fatal("esperaba error con hash inválido")
	}
}

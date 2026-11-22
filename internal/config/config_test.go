package config

import "testing"

func TestLoad(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("BASE_DOMAIN", "")
	t.Setenv("DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("esperaba error sin variables")
	}

	t.Setenv("APP_ENV", "staging")
	t.Setenv("BASE_DOMAIN", "localhost")
	t.Setenv("DATABASE_URL", "postgres://x")
	if _, err := Load(); err == nil {
		t.Fatal("esperaba error con APP_ENV inválido")
	}

	t.Setenv("APP_ENV", "dev")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":8080" {
		t.Fatalf("Addr = %q", c.Addr)
	}
}

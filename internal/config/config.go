// Package config lee y valida las variables de entorno al arrancar.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Env         string // dev | prod
	Addr        string
	BaseDomain  string
	DatabaseURL string
}

// Load falla si falta una variable obligatoria: el binario no debe iniciar a medias.
func Load() (Config, error) {
	c := Config{
		Env:         os.Getenv("APP_ENV"),
		Addr:        os.Getenv("ADDR"),
		BaseDomain:  strings.ToLower(os.Getenv("BASE_DOMAIN")),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}
	if c.Addr == "" {
		c.Addr = ":8080"
	}
	var errs []error
	for name, v := range map[string]string{"APP_ENV": c.Env, "BASE_DOMAIN": c.BaseDomain, "DATABASE_URL": c.DatabaseURL} {
		if v == "" {
			errs = append(errs, fmt.Errorf("falta %s", name))
		}
	}
	if c.Env != "" && c.Env != "dev" && c.Env != "prod" {
		errs = append(errs, fmt.Errorf("APP_ENV inválido: %q (dev|prod)", c.Env))
	}
	return c, errors.Join(errs...)
}

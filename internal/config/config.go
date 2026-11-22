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

	// R2: si falta alguna, en desarrollo se guarda en disco (uploads/); en producción es obligatorio.
	R2AccountID, R2AccessKey, R2SecretKey, R2Bucket, R2PublicURL string
}

func (c Config) HasR2() bool {
	return c.R2AccountID != "" && c.R2AccessKey != "" && c.R2SecretKey != "" && c.R2Bucket != "" && c.R2PublicURL != ""
}

// Load falla si falta una variable obligatoria: el binario no debe iniciar a medias.
func Load() (Config, error) {
	c := Config{
		Env:         os.Getenv("APP_ENV"),
		Addr:        os.Getenv("ADDR"),
		BaseDomain:  strings.ToLower(os.Getenv("BASE_DOMAIN")),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		R2AccountID: os.Getenv("R2_ACCOUNT_ID"),
		R2AccessKey: os.Getenv("R2_ACCESS_KEY_ID"),
		R2SecretKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
		R2Bucket:    os.Getenv("R2_BUCKET"),
		R2PublicURL: strings.TrimSuffix(os.Getenv("R2_PUBLIC_URL"), "/"),
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
	if c.Env == "prod" && !c.HasR2() {
		errs = append(errs, errors.New("en producción faltan las variables R2_*"))
	}
	return c, errors.Join(errs...)
}

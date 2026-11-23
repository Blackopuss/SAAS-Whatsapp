// createadmin da de alta un superadmin con 2FA TOTP.
//
//	go run ./cmd/createadmin -email yo@ejemplo.com < password.txt
//	go run ./cmd/createadmin -reset -email yo@ejemplo.com < password.txt  (perdiste la clave: contraseña y clave nuevas)
//
// Necesita DATABASE_URL y TOTP_KEY. Imprime la clave para tu app de autenticación.
package main

import (
	"bufio"
	"context"
	"encoding/base32"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/auth"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
)

func main() {
	email := flag.String("email", "", "email del superadmin")
	reset := flag.Bool("reset", false, "cambiar contraseña y clave TOTP de un superadmin que ya existe")
	flag.Parse()
	if err := run(strings.ToLower(strings.TrimSpace(*email)), *reset); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(email string, reset bool) error {
	key, err := hex.DecodeString(os.Getenv("TOTP_KEY"))
	if err != nil || len(key) != 32 {
		return fmt.Errorf("TOTP_KEY debe ser 64 caracteres hex (openssl rand -hex 32)")
	}
	if !strings.Contains(email, "@") {
		return fmt.Errorf("falta -email")
	}
	fmt.Fprint(os.Stderr, "Contraseña (mínimo 12 caracteres): ")
	pass, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && pass == "" {
		return err
	}
	pass = strings.TrimRight(pass, "\r\n")
	if len(pass) < 12 {
		return fmt.Errorf("la contraseña debe tener al menos 12 caracteres")
	}
	hash, err := auth.Hash(pass)
	if err != nil {
		return err
	}
	secret := auth.NewTOTPSecret()
	sealed, err := auth.Seal(key, secret)
	if err != nil {
		return err
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()
	verb := "creado"
	if reset {
		n, err := db.New(conn).ResetAdmin(ctx, db.ResetAdminParams{Email: email, PasswordHash: hash, TotpSecret: sealed})
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("no existe el superadmin %s", email)
		}
		verb = "actualizado (la clave anterior ya no sirve)"
	} else if _, err := db.New(conn).CreateAdmin(ctx, db.CreateAdminParams{Email: email, PasswordHash: hash, TotpSecret: sealed}); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "\nSuperadmin %s %s. Agrega esta clave en tu app de autenticación (Google Authenticator, 1Password...):\n\n", email, verb)
	fmt.Println(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret))
	fmt.Fprintf(os.Stderr, "\nO como enlace: %s\n", auth.TOTPURI(secret, "Tiendas", email))
	return nil
}

// createstore da de alta una tienda y su vendedora mientras no exista el superadmin (sprint 5).
//
//	go run ./cmd/createstore -slug ana -name "Ana Moda" -email ana@ejemplo.com < password.txt
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/auth"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/web"
)

func main() {
	slug := flag.String("slug", "", "subdominio de la tienda (a-z0-9-, 3 a 30)")
	name := flag.String("name", "", "nombre comercial")
	email := flag.String("email", "", "email de la vendedora")
	flag.Parse()
	if err := run(*slug, *name, strings.ToLower(strings.TrimSpace(*email))); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(slug, name, email string) error {
	if !web.ValidSlug(slug) || name == "" || !strings.Contains(email, "@") {
		return fmt.Errorf("revisa -slug (a-z0-9-, 3 a 30, no reservado), -name y -email")
	}
	fmt.Fprint(os.Stderr, "Contraseña (mínimo 10 caracteres): ")
	pass, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && pass == "" {
		return err
	}
	pass = strings.TrimRight(pass, "\r\n")
	if len(pass) < 10 {
		return fmt.Errorf("la contraseña debe tener al menos 10 caracteres")
	}
	hash, err := auth.Hash(pass)
	if err != nil {
		return err
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)
	s, err := q.CreateStore(ctx, db.CreateStoreParams{Slug: slug, Name: name})
	if err != nil {
		return err
	}
	if _, err := q.CreateUser(ctx, db.CreateUserParams{StoreID: s.ID, Email: email, PasswordHash: hash}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "\nTienda %q creada (id %d)\n", slug, s.ID)
	return nil
}

package web

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/auth"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
)

// TestStoreIsolation (plan, sprint 1 y 2): la vendedora A intenta leer, editar y borrar datos de B por ID
// y recibe 404. Corre contra Postgres real; se salta si no hay TEST_DATABASE_URL.
func TestStoreIsolation(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("sin TEST_DATABASE_URL")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	q := db.New(pool)

	suffix := strconv.FormatInt(time.Now().UnixNano()%1e9, 36)
	hash, _ := auth.Hash("contraseña-segura")
	mk := func(slug string) db.Store {
		s, err := q.CreateStore(ctx, db.CreateStoreParams{Slug: slug, Name: slug})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := q.CreateUser(ctx, db.CreateUserParams{StoreID: s.ID, Email: slug + "@x.mx", PasswordHash: hash}); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(ctx, "DELETE FROM stores WHERE id = $1", s.ID) })
		return s
	}
	a, b := mk("iso-a-"+suffix), mk("iso-b-"+suffix)

	bCat, err := q.CreateCategory(ctx, db.CreateCategoryParams{StoreID: b.ID, Name: "De B"})
	if err != nil {
		t.Fatal(err)
	}
	bProd, err := q.CreateProduct(ctx, db.CreateProductParams{StoreID: b.ID, CategoryID: pgtype.Int8{Int64: bCat.ID, Valid: true}, Name: "Producto de B", PriceCents: 10000})
	if err != nil {
		t.Fatal(err)
	}

	h := Routes(Deps{
		BaseDomain: "tiendas.test", DB: pool, Q: q,
		Tx: func(ctx context.Context, fn func(db.Querier) error) error { return db.InTx(ctx, pool, fn) },
	})
	aHost := "http://" + a.Slug + ".tiendas.test"
	cookie := login(t, h, a.Slug+".tiendas.test", a.Slug+"@x.mx")
	pid, cid := strconv.FormatInt(bProd, 10), strconv.FormatInt(bCat.ID, 10)

	for _, c := range []struct {
		method, path string
		form         url.Values
	}{
		{"GET", "/panel/productos/" + pid, nil},
		{"POST", "/panel/productos/" + pid, url.Values{"name": {"hackeado"}, "price": {"1"}}},
		{"POST", "/panel/productos/" + pid + "/borrar", url.Values{}},
		{"POST", "/panel/productos/" + pid + "/marcar/active", url.Values{}},
		{"POST", "/panel/categorias/" + cid + "/renombrar", url.Values{"name": {"hackeada"}}},
		{"POST", "/panel/categorias/" + cid + "/borrar", url.Values{}},
		{"POST", "/panel/categorias/" + cid + "/mover", url.Values{"dir": {"up"}}},
	} {
		if rec := do(h, c.method, aHost+c.path, c.form, cookie); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s como A: status %d, quiero 404", c.method, c.path, rec.Code)
		}
	}

	// Crear un producto de A colgado de la categoría de B: la FK compuesta lo impide.
	rec := do(h, "POST", aHost+"/panel/productos", url.Values{"name": {"x"}, "price": {"1"}, "category_id": {cid}}, cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("producto de A con categoría de B: status %d, quiero 422", rec.Code)
	}

	// Lo de B sigue intacto.
	p, err := q.GetProduct(ctx, db.GetProductParams{StoreID: b.ID, ID: bProd})
	if err != nil || p.Name != "Producto de B" || !p.Active {
		t.Fatalf("producto de B alterado: %+v %v", p, err)
	}
	cats, _ := q.ListCategories(ctx, b.ID)
	if len(cats) != 1 || cats[0].Name != "De B" {
		t.Fatalf("categoría de B alterada: %+v", cats)
	}

	// Y A sí puede crear y editar lo suyo, con variantes.
	rec = do(h, "POST", aHost+"/panel/productos", url.Values{"name": {"Blusa"}, "price": {"289.50"}, "sizes": {"S, M, L"}, "colors": {"Negro, Blanco"}}, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("crear producto de A: status %d: %s", rec.Code, rec.Body.String())
	}
	ps, _ := q.ListProducts(ctx, a.ID)
	if len(ps) != 1 || ps[0].PriceCents != 28950 {
		t.Fatalf("productos de A: %+v", ps)
	}
	vs, _ := q.ListVariants(ctx, db.ListVariantsParams{StoreID: a.ID, ProductID: ps[0].ID})
	if len(vs) != 6 {
		t.Fatalf("variantes = %d, quiero 6", len(vs))
	}
	// Marcar una variante agotada se conserva al guardar de nuevo.
	aPid := fmt.Sprint(ps[0].ID)
	do(h, "POST", aHost+"/panel/productos/"+aPid, url.Values{"name": {"Blusa"}, "price": {"289.50"}, "sizes": {"S, M, L"}, "colors": {"Negro, Blanco"}, "soldout": {"M|Negro"}}, cookie)
	vs, _ = q.ListVariants(ctx, db.ListVariantsParams{StoreID: a.ID, ProductID: ps[0].ID})
	soldOut := 0
	for _, v := range vs {
		if v.SoldOut {
			soldOut++
			if v.Size != "M" || v.Color != "Negro" {
				t.Errorf("variante agotada equivocada: %+v", v)
			}
		}
	}
	if soldOut != 1 {
		t.Errorf("agotadas = %d, quiero 1", soldOut)
	}
}

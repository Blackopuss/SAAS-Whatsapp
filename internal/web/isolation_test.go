package web

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/auth"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/storage"
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
	mk := func(slug string) db.CreateStoreRow {
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
	bProd, err := q.CreateProduct(ctx, db.CreateProductParams{StoreID: b.ID, CategoryID: pgtype.Int8{Int64: bCat.ID, Valid: true}, Name: "Producto de B", PriceCents: 10000, Unit: "pieza", Step: 1000})
	if err != nil {
		t.Fatal(err)
	}

	disk := storage.Disk{Dir: t.TempDir()}
	h := Routes(Deps{
		BaseDomain: "tiendas.test", DB: pool, Q: q, Storage: disk, MediaURL: "/media/",
		Tx: func(ctx context.Context, fn func(db.Querier) error) error { return db.InTx(ctx, pool, fn) },
	})
	bImg, err := q.CreateProductImage(ctx, db.CreateProductImageParams{StoreID: b.ID, ProductID: bProd, R2Key: storage.NewBaseKey(b.ID), Widths: []int32{400}})
	if err != nil {
		t.Fatal(err)
	}
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

	// Tienda pública: el producto de B no se ve desde el subdominio de A, ni en su carrito.json.
	if rec := do(h, "GET", aHost+"/p/"+pid, nil, nil); rec.Code != http.StatusNotFound {
		t.Errorf("producto de B en tienda A: status %d", rec.Code)
	}
	if rec := do(h, "GET", aHost+"/c/"+cid, nil, nil); rec.Code != http.StatusNotFound {
		t.Errorf("categoría de B en tienda A: status %d", rec.Code)
	}
	if rec := do(h, "GET", aHost+"/carrito.json", nil, nil); rec.Code != 200 || strings.Contains(rec.Body.String(), "Producto de B") {
		t.Errorf("carrito.json de A: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, "GET", "http://"+b.Slug+".tiendas.test/p/"+pid, nil, nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), "Producto de B") {
		t.Errorf("producto de B en su tienda: status %d", rec.Code)
	}

	// Fotos: subir al producto de B o borrar la foto de B también da 404.
	if rec := upload(h, aHost+"/panel/productos/"+pid+"/fotos", pngBytes(), cookie); rec.Code != http.StatusNotFound {
		t.Errorf("subir foto a producto de B: status %d", rec.Code)
	}
	if rec := do(h, "POST", aHost+"/panel/fotos/"+strconv.FormatInt(bImg, 10)+"/borrar", url.Values{}, cookie); rec.Code != http.StatusNotFound {
		t.Errorf("borrar foto de B: status %d", rec.Code)
	}
	if n, _ := q.CountProductImages(ctx, db.CountProductImagesParams{StoreID: b.ID, ProductID: bProd}); n != 1 {
		t.Errorf("fotos de B = %d, quiero 1", n)
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

	// Subida de fotos de A: un archivo falso se rechaza, una foto real se guarda recodificada.
	photos := aHost + "/panel/productos/" + aPid + "/fotos"
	if rec := upload(h, photos, []byte("<?php system($_GET[1]); ?>"), cookie); rec.Header().Get("Location") != "/panel/productos/"+aPid+"?foto=invalida#fotos" {
		t.Errorf("archivo falso: Location %q", rec.Header().Get("Location"))
	}
	if rec := upload(h, photos, pngBytes(), cookie); rec.Header().Get("Location") != "/panel/productos/"+aPid+"?foto=ok#fotos" {
		t.Fatalf("foto válida: status %d Location %q", rec.Code, rec.Header().Get("Location"))
	}
	imgs, _ := q.ListProductImages(ctx, db.ListProductImagesParams{StoreID: a.ID, ProductID: ps[0].ID})
	if len(imgs) != 1 || len(imgs[0].Widths) == 0 || !strings.HasPrefix(imgs[0].R2Key, "stores/"+strconv.FormatInt(a.ID, 10)+"/") {
		t.Fatalf("imagen guardada: %+v", imgs)
	}
	for _, k := range photoKeys(imgs[0].R2Key, imgs[0].Widths) {
		if _, err := os.Stat(filepath.Join(disk.Dir, filepath.FromSlash(k))); err != nil {
			t.Errorf("falta archivo %s", k)
		}
	}
	// Borrar el producto borra también sus archivos.
	do(h, "POST", aHost+"/panel/productos/"+aPid+"/borrar", url.Values{}, cookie)
	for _, k := range photoKeys(imgs[0].R2Key, imgs[0].Widths) {
		if _, err := os.Stat(filepath.Join(disk.Dir, filepath.FromSlash(k))); err == nil {
			t.Errorf("quedó el archivo %s", k)
		}
	}
}

func pngBytes() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 500, 625))
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func upload(h http.Handler, target string, data []byte, cookie *http.Cookie) *httptest.ResponseRecorder {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("foto", "foto.png")
	_, _ = fw.Write(data)
	_ = mw.Close()
	r := httptest.NewRequest("POST", target, &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

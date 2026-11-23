// seeddemo llena una tienda existente con 20 productos de ejemplo (fotos generadas) para medir la plantilla.
//
//	go run ./cmd/seeddemo -slug ana
//
// Solo para desarrollo: guarda las fotos en uploads/ y borra antes el catálogo de esa tienda.
// Si pones fotos en demo/fotos/ (JPG, PNG o WebP) las usa en orden; si no, genera siluetas.
package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"image/color"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/config"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/images"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/storage"
)

type item struct {
	cat, name, desc string
	price, sale     int32
	sizes, colors   string
	hue             float64
	isNew           bool
}

var catalog = []item{
	{"Blusas", "Blusa de lino", "Lino fresco, ideal para calor.\nCorte recto, botones de concha.", 28900, 24900, "S,M,L", "Blanco,Negro", 30, true},
	{"Blusas", "Blusa bordada Oaxaca", "Bordado a mano en el cuello.", 45000, 0, "S,M,L,XL", "Blanco", 350, true},
	{"Blusas", "Blusa de satín", "Caída suave, se ve elegante.", 32000, 0, "S,M", "Rosa,Vino", 340, false},
	{"Blusas", "Top tejido", "Tejido de punto, elástico.", 19900, 15900, "CH,M,G", "Beige,Negro", 40, false},
	{"Blusas", "Camisa a cuadros", "Algodón, manga larga.", 34900, 0, "S,M,L", "Rojo,Azul", 0, false},
	{"Vestidos", "Vestido midi floral", "Estampado floral, largo a media pierna.", 52000, 0, "S,M,L", "Rosa,Azul", 330, true},
	{"Vestidos", "Vestido negro básico", "El que combina con todo.", 39900, 34900, "S,M,L,XL", "Negro", 270, false},
	{"Vestidos", "Vestido de manta", "Manta natural con listón.", 48000, 0, "Unitalla", "Crema", 50, false},
	{"Vestidos", "Vestido camisero", "Con cinturón del mismo tela.", 46000, 0, "M,L", "Mostaza,Verde", 60, true},
	{"Vestidos", "Vestido de fiesta", "Brillo discreto, forro completo.", 89000, 75000, "S,M", "Morado,Negro", 290, false},
	{"Pantalones", "Jeans tiro alto", "Mezclilla con stretch.", 49900, 0, "26,28,30,32", "Mezclilla", 220, false},
	{"Pantalones", "Pantalón palazzo", "Pierna amplia, muy fresco.", 42000, 0, "S,M,L", "Negro,Beige", 30, true},
	{"Pantalones", "Pantalón cargo", "Bolsas laterales, gabardina.", 45000, 39000, "28,30,32", "Olivo,Arena", 90, false},
	{"Pantalones", "Short de mezclilla", "Deshilado en la orilla.", 25000, 0, "26,28,30", "Mezclilla", 210, false},
	{"Pantalones", "Falda plisada", "Plisado permanente, cintura elástica.", 36000, 0, "Unitalla", "Rosa,Gris", 320, false},
	{"Accesorios", "Bolsa de palma", "Tejida a mano en Yucatán.", 38000, 0, "", "", 45, true},
	{"Accesorios", "Aretes de chaquira", "Hechos por artesanas huicholas.", 22000, 0, "", "Azul,Rojo", 200, false},
	{"Accesorios", "Rebozo de algodón", "Rebozo tradicional, 2 m.", 65000, 59000, "", "Turquesa,Coral", 170, false},
	{"Accesorios", "Sombrero de ala ancha", "Para el sol, ajustable.", 29000, 0, "", "Camel", 35, false},
	{"Accesorios", "Collar de plata", "Plata .925 de Taxco.", 54000, 0, "", "Plateado", 250, false},
}

func main() {
	slug := flag.String("slug", "ana", "tienda a llenar")
	dir := flag.String("fotos", "demo/fotos", "carpeta con fotos (vacío = siluetas generadas)")
	flag.Parse()
	if *dir != "" {
		realPhotos, _ = filepath.Glob(filepath.Join(*dir, "*"))
	}
	if err := run(*slug); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(slug string) error {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer pool.Close()
	q := db.New(pool)
	s, err := q.GetStoreBySlug(ctx, slug)
	if err != nil {
		return fmt.Errorf("tienda %q: %w", slug, err)
	}
	// Mismo destino que el servidor: R2 si hay variables R2_*, si no uploads/.
	var disk storage.Storage = storage.Disk{Dir: "uploads"}
	if cfg, _ := config.Load(); cfg.HasR2() {
		if disk, err = storage.NewR2(cfg.R2AccountID, cfg.R2AccessKey, cfg.R2SecretKey, cfg.R2Bucket); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "fotos a R2:", cfg.R2Bucket)
	}

	// Empieza limpio: borra pedidos, productos (y sus archivos) y categorías de esta tienda.
	if _, err := pool.Exec(ctx, "DELETE FROM orders WHERE store_id = $1", s.ID); err != nil {
		return err
	}
	old, err := q.ListProducts(ctx, s.ID)
	if err != nil {
		return err
	}
	for _, p := range old {
		imgs, _ := q.ListProductImages(ctx, db.ListProductImagesParams{StoreID: s.ID, ProductID: p.ID})
		for _, im := range imgs {
			for _, w := range im.Widths {
				_ = disk.Delete(ctx, storage.VariantKey(im.R2Key, w, "webp"))
			}
			_ = disk.Delete(ctx, storage.OGKey(im.R2Key))
		}
		if _, err := q.DeleteProduct(ctx, db.DeleteProductParams{StoreID: s.ID, ID: p.ID}); err != nil {
			return err
		}
	}
	cats, _ := q.ListCategories(ctx, s.ID)
	for _, c := range cats {
		_, _ = q.DeleteCategory(ctx, db.DeleteCategoryParams{StoreID: s.ID, ID: c.ID})
	}

	catIDs := map[string]int64{}
	for _, it := range catalog {
		if catIDs[it.cat] == 0 {
			c, err := q.CreateCategory(ctx, db.CreateCategoryParams{StoreID: s.ID, Name: it.cat})
			if err != nil {
				return err
			}
			catIDs[it.cat] = c.ID
		}
	}
	for i := len(catalog) - 1; i >= 0; i-- { // al revés: el primero de la lista queda como el más nuevo
		it := catalog[i]
		sale := pgtype.Int4{Int32: it.sale, Valid: it.sale > 0}
		pid, err := q.CreateProduct(ctx, db.CreateProductParams{
			StoreID: s.ID, CategoryID: pgtype.Int8{Int64: catIDs[it.cat], Valid: true},
			Name: it.name, Description: it.desc, PriceCents: it.price, SalePriceCents: sale,
			Unit: "pieza", Step: 1000, Stock: pgtype.Int4{Int32: int32(3+i%6) * 1000, Valid: it.sizes == "" && it.colors == ""}, // #nosec G115 -- índice chico
		})
		if err != nil {
			return err
		}
		for flag, on := range map[string]bool{"is_new": it.isNew, "on_sale": it.sale > 0, "sold_out": i == 13} {
			if on {
				if _, err := q.ToggleProductFlag(ctx, db.ToggleProductFlagParams{StoreID: s.ID, ID: pid, Flag: flag}); err != nil {
					return err
				}
			}
		}
		for si, size := range split(it.sizes) {
			for ci, col := range split(it.colors) {
				stock := int32((si*3+ci*2+i)%6) * 1000 // #nosec G115 -- índices chicos; algunas quedan en 0 (agotadas)
				if si == 0 && ci == 1 {
					stock = 0
				}
				if err := q.CreateVariant(ctx, db.CreateVariantParams{StoreID: s.ID, ProductID: pid, Size: size, Color: col, Stock: pgtype.Int4{Int32: stock, Valid: true}}); err != nil {
					return err
				}
			}
		}
		for n := range 1 + i%3 {
			if err := addPhoto(ctx, q, disk, s.ID, pid, it.hue+float64(n)*25); err != nil {
				return err
			}
		}
		fmt.Fprintf(os.Stderr, ".")
	}
	fmt.Fprintf(os.Stderr, "\n%d productos en %s\n", len(catalog), slug)
	return seedOrders(ctx, pool, q, s.ID)
}

// seedOrders llena 90 días de pedidos de ejemplo para ver las gráficas de Ventas. No toca existencias.
func seedOrders(ctx context.Context, pool *pgxpool.Pool, q *db.Queries, storeID int64) error {
	all, err := q.ListPanelProducts(ctx, storeID)
	if err != nil {
		return err
	}
	ids := make([]int64, 0, len(all))
	for _, p := range all {
		ids = append(ids, p.ID)
	}
	ps, err := q.OrderProducts(ctx, db.OrderProductsParams{StoreID: storeID, Ids: ids})
	if err != nil || len(ps) == 0 {
		return err
	}
	vs, err := q.OrderVariants(ctx, db.OrderVariantsParams{StoreID: storeID, Ids: ids})
	if err != nil {
		return err
	}
	combos := map[int64][]db.OrderVariantsRow{}
	for _, v := range vs {
		combos[v.ProductID] = append(combos[v.ProductID], v)
	}
	rng := rand.New(rand.NewPCG(7, uint64(storeID))) // #nosec G404 G115 -- datos de ejemplo; storeID es un id positivo
	const letters = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	for n := range 70 {
		code := make([]byte, 6)
		for i := range code {
			code[i] = letters[rng.IntN(len(letters))]
		}
		var total int64
		var items []db.AddOrderItemParams
		for range 1 + rng.IntN(3) {
			p := ps[rng.IntN(len(ps))]
			it := db.AddOrderItemParams{StoreID: storeID, ProductID: pgtype.Int8{Int64: p.ID, Valid: true}, Name: p.Name, Category: p.Category,
				Unit: p.Unit, Qty: int32(1+rng.IntN(2)) * 1000, PriceCents: p.PriceCents} // #nosec G115 -- 1 o 2
			if cs := combos[p.ID]; len(cs) > 0 {
				c := cs[rng.IntN(len(cs))]
				it.Size, it.Color = c.Size, c.Color
			}
			total += (int64(it.PriceCents)*int64(it.Qty) + 500) / 1000
			items = append(items, it)
		}
		id, err := q.CreateOrder(ctx, db.CreateOrderParams{StoreID: storeID, Code: string(code), TotalCents: int32(total)}) // #nosec G115 -- montos chicos
		if err != nil {
			continue // código repetido: se salta
		}
		for _, it := range items {
			it.OrderID = id
			if err := q.AddOrderItem(ctx, it); err != nil {
				return err
			}
		}
		// Los más recientes quedan pendientes; de los demás, 7 de cada 10 se venden.
		status, reason := 1, 0
		switch {
		case n < 4:
			status = 0
		case rng.IntN(10) >= 7:
			status, reason = 2, 1+rng.IntN(4)
		}
		days := n * 90 / 70
		if _, err := pool.Exec(ctx, `UPDATE orders SET status = $3::smallint, reason = $4::smallint,
			created_at = now() - make_interval(days => $5::int, hours => $6::int),
			decided_at = CASE WHEN $3::smallint = 0 THEN NULL ELSE now() - make_interval(days => $5::int) END
			WHERE store_id = $1 AND id = $2`, storeID, id, status, reason, days, rng.IntN(12)); err != nil {
			return err
		}
	}
	fmt.Fprintln(os.Stderr, "pedidos de ejemplo listos")
	return nil
}

// split devuelve [""] si no hay valores: así el producto sin tallas ni colores no tiene variantes cruzadas vacías.
func split(s string) []string {
	if s == "" {
		return []string{""}
	}
	return strings.Split(s, ",")
}

// realPhotos: si hay fotos en -fotos se usan en orden (una por producto, en ciclo) en vez de las generadas.
var realPhotos []string

var photoN int

func addPhoto(ctx context.Context, q *db.Queries, st storage.Storage, storeID, pid int64, hue float64) error {
	img := fakePhoto(hue)
	if len(realPhotos) > 0 {
		f := realPhotos[photoN%len(realPhotos)]
		photoN++
		data, err := os.ReadFile(f) // #nosec G304 -- herramienta local de desarrollo
		if err != nil {
			return err
		}
		if img, err = images.Decode(data); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
	}
	rs, err := images.Renditions(img)
	if err != nil {
		return err
	}
	base := storage.NewBaseKey(storeID)
	var widths []int32
	for _, r := range rs {
		w := int32(r.Width) // #nosec G115 -- anchos de 400 a 1200
		if err := st.Put(ctx, storage.VariantKey(base, w, r.Format), r.Data, "image/webp"); err != nil {
			return err
		}
		widths = append(widths, w)
	}
	og, err := images.OGJPEG(img)
	if err != nil {
		return err
	}
	if err := st.Put(ctx, storage.OGKey(base), og, "image/jpeg"); err != nil {
		return err
	}
	_, err = q.CreateProductImage(ctx, db.CreateProductImageParams{StoreID: storeID, ProductID: pid, R2Key: base, DominantColor: images.DominantColor(img), Widths: widths})
	return err
}

// fakePhoto pinta una "prenda" (silueta suave) sobre un fondo de estudio, en 1000×1250.
func fakePhoto(hue float64) image.Image {
	const w, h = 1000, 1250
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fg := hsl(hue, 0.55, 0.5)
	for y := range h {
		for x := range w {
			fy, fx := float64(y)/h, float64(x)/w
			c := hsl(hue+20, 0.18, 0.9-0.08*fy)
			half := 0.18 + 0.12*fy // silueta que se abre hacia abajo
			if fy > 0.15 && fy < 0.9 && math.Abs(fx-0.5) < half {
				shade := 0.85 + 0.15*math.Sin(fx*40+fy*6)
				c = color.RGBA{uint8(float64(fg.R) * shade), uint8(float64(fg.G) * shade), uint8(float64(fg.B) * shade), 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func hsl(h, s, l float64) color.RGBA {
	h = math.Mod(h, 360) / 360
	f := func(n float64) uint8 {
		k := math.Mod(n+h*12, 12)
		a := s * math.Min(l, 1-l)
		return uint8(255 * (l - a*math.Max(-1, math.Min(math.Min(k-3, 9-k), 1))))
	}
	return color.RGBA{f(0), f(8), f(4), 255}
}

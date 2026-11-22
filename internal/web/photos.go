package web

import (
	"context"
	"errors"
	"image"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"

	"github.com/Blackopuss/SAAS-Whatsapp/internal/db"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/images"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/storage"
	"github.com/Blackopuss/SAAS-Whatsapp/internal/web/views"
)

const maxPhotos = 10

// shortcut: las fotos se procesan en la petición (unos segundos), con 2 a la vez para no ahogar el CPU.
// Mover a un job de River (plan, sprint 2) si las subidas tardan demasiado o el VPS se satura.
var photoSlots = make(chan struct{}, 2)

var photoErrors = map[string]string{
	"limite":   "Ya tiene 10 fotos, que es el máximo por producto. Borra una para subir otra.",
	"invalida": images.ErrInvalid.Error() + ".",
	"falta":    "Elige una foto para subir.",
	"pesada":   "La foto debe pesar menos de 10 MB y medir menos de 40 megapíxeles.",
}

func photoKeys(base string, widths []int32) []string {
	keys := []string{storage.OGKey(base)}
	for _, w := range widths {
		keys = append(keys, storage.VariantKey(base, w, "webp"))
	}
	return keys
}

func (a *app) photoUpload(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	ctx, s := r.Context(), StoreFrom(r.Context())
	editURL := "/panel/productos/" + strconv.FormatInt(id, 10)
	fail := func(code string) { back(w, r, editURL+"?foto="+url.QueryEscape(code)+"#fotos") }

	if _, err := a.q.GetProduct(ctx, db.GetProductParams{StoreID: s.ID, ID: id}); err != nil {
		a.notFoundOr500(w, r, err)
		return
	}
	n, err := a.q.CountProductImages(ctx, db.CountProductImagesParams{StoreID: s.ID, ProductID: id})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if n >= maxPhotos {
		fail("limite")
		return
	}

	up, code, err := a.saveUpload(r, "foto", s.ID, 0)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if code != "" {
		fail(code)
		return
	}
	if _, err := a.q.CreateProductImage(ctx, db.CreateProductImageParams{StoreID: s.ID, ProductID: id, R2Key: up.base, DominantColor: images.DominantColor(up.img), Widths: up.widths}); err != nil {
		a.cleanup(ctx, up.keys)
		a.fail(w, r, err)
		return
	}
	back(w, r, editURL+"?foto=ok#fotos")
}

func (a *app) photoDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	row, err := a.q.DeleteProductImage(r.Context(), db.DeleteProductImageParams{StoreID: StoreFrom(r.Context()).ID, ID: id})
	if err != nil {
		a.notFoundOr500(w, r, err)
		return
	}
	a.cleanup(r.Context(), photoKeys(row.R2Key, row.Widths))
	back(w, r, "/panel/productos/"+strconv.FormatInt(row.ProductID, 10)+"#fotos")
}

// cleanup borra archivos; si falla solo queda basura en el almacenamiento, nunca datos perdidos.
func (a *app) cleanup(ctx context.Context, keys []string) {
	if err := a.store.Delete(context.WithoutCancel(ctx), keys...); err != nil {
		Log(ctx).Error("borrar archivos", "err", err)
	}
}

// photoViews arma las rutas de cada foto para la vista.
func (a *app) photoViews(imgs []db.ProductImage) []views.Photo {
	var out []views.Photo
	for _, im := range imgs {
		p := views.Photo{ID: im.ID}
		for _, w := range im.Widths {
			ws := strconv.Itoa(int(w))
			p.SrcSet += a.mediaURL + storage.VariantKey(im.R2Key, w, "webp") + " " + ws + "w, "
		}
		if len(im.Widths) > 0 {
			p.Src = a.mediaURL + storage.VariantKey(im.R2Key, im.Widths[0], "webp")
		}
		out = append(out, p)
	}
	return out
}

type stored struct {
	base   string
	widths []int32
	keys   []string
	img    image.Image
}

// saveUpload lee el archivo del campo, lo valida, lo recodifica a WebP y lo guarda.
// code != "" es un error del usuario (clave de photoErrors); err es un error interno.
// maxRenditions > 0 limita cuántos anchos se guardan (el logo solo necesita 400 px).
func (a *app) saveUpload(r *http.Request, field string, storeID int64, maxRenditions int) (stored, string, error) {
	ctx := r.Context()
	file, _, err := r.FormFile(field)
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return stored{}, "pesada", nil
		}
		return stored{}, "falta", nil
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, images.MaxBytes+1))
	if err != nil {
		return stored{}, "falta", nil
	}
	img, err := images.Decode(data)
	if err != nil {
		Log(ctx).Warn("imagen rechazada", "err", err)
		if errors.Is(err, images.ErrInvalid) {
			return stored{}, "invalida", nil
		}
		return stored{}, "pesada", nil
	}

	select {
	case photoSlots <- struct{}{}:
		defer func() { <-photoSlots }()
	case <-ctx.Done():
		return stored{}, "", ctx.Err()
	}
	rends, err := images.Renditions(img)
	if err != nil {
		return stored{}, "", err
	}
	if maxRenditions > 0 && len(rends) > maxRenditions {
		rends = rends[:maxRenditions]
	}
	up := stored{base: storage.NewBaseKey(storeID), img: img}
	for _, rd := range rends {
		w32 := int32(rd.Width) // #nosec G115 -- anchos acotados a 1200
		key := storage.VariantKey(up.base, w32, rd.Format)
		if err := a.store.Put(ctx, key, rd.Data, "image/"+rd.Format); err != nil {
			a.cleanup(ctx, up.keys)
			return stored{}, "", err
		}
		up.keys = append(up.keys, key)
		if !slices.Contains(up.widths, w32) {
			up.widths = append(up.widths, w32)
		}
	}
	og, err := images.OGJPEG(img)
	if err == nil {
		err = a.store.Put(ctx, storage.OGKey(up.base), og, "image/jpeg")
	}
	if err != nil {
		a.cleanup(ctx, up.keys)
		return stored{}, "", err
	}
	up.keys = append(up.keys, storage.OGKey(up.base))
	return up, "", nil
}

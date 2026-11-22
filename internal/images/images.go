// Package images valida y recodifica las fotos de producto. Recodificar siempre elimina EXIF (con GPS)
// y cualquier payload escondido: solo sobreviven los píxeles.
package images

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg" // registra decodificadores
	_ "image/png"
	"net/http"

	"github.com/gen2brain/avif"
	"github.com/gen2brain/webp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const MaxBytes = 10 << 20 // 10 MB por foto

// maxPixels frena "bombas" de descompresión: un PNG chico que dice medir 50 000 × 50 000.
var maxPixels = 40_000_000

var Widths = []int{400, 800, 1200}

var allowed = map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true}

var ErrInvalid = errors.New("el archivo no es una foto JPG, PNG o WebP válida")

type Rendition struct {
	Width  int
	Format string // "webp" | "avif"
	Data   []byte
}

// Decode valida el tipo real por magic bytes (no por extensión ni Content-Type) y el tamaño antes de decodificar.
// shortcut: no aplica la orientación EXIF de JPEG; el navegador ya entrega WebP rotado. Si se aceptan subidas
// sin comprimir en el cliente, leer EXIF aquí.
func Decode(data []byte) (image.Image, error) {
	if len(data) == 0 || len(data) > MaxBytes {
		return nil, fmt.Errorf("la foto debe pesar menos de 10 MB")
	}
	if !allowed[http.DetectContentType(data)] {
		return nil, ErrInvalid
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, ErrInvalid
	}
	if cfg.Width*cfg.Height > maxPixels {
		return nil, fmt.Errorf("la foto es demasiado grande (máximo 40 megapíxeles)")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrInvalid
	}
	return img, nil
}

// Renditions genera 400, 800 y 1200 px de ancho en WebP y AVIF, sin agrandar fotos pequeñas.
func Renditions(img image.Image) ([]Rendition, error) {
	var out []Rendition
	b := img.Bounds()
	for _, w := range Widths {
		last := w >= b.Dx() // esta versión ya es del tamaño original: no inventar píxeles con anchos mayores
		w = min(w, b.Dx())
		h := max(1, b.Dy()*w/b.Dx())
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)

		var wb, ab bytes.Buffer
		if err := webp.Encode(&wb, dst, webp.Options{Quality: 78, Method: 4}); err != nil {
			return nil, err
		}
		if err := avif.Encode(&ab, dst, avif.Options{Quality: 55, Speed: 8}); err != nil {
			return nil, err
		}
		out = append(out, Rendition{w, "webp", wb.Bytes()}, Rendition{w, "avif", ab.Bytes()})
		if last {
			break
		}
	}
	return out, nil
}

// DominantColor promedia la foto reducida a 8×8; sirve de placeholder mientras carga.
func DominantColor(img image.Image) string {
	small := image.NewRGBA(image.Rect(0, 0, 8, 8))
	draw.ApproxBiLinear.Scale(small, small.Bounds(), img, img.Bounds(), draw.Src, nil)
	var r, g, b int
	for y := range 8 {
		for x := range 8 {
			c := color.RGBAModel.Convert(small.At(x, y)).(color.RGBA)
			r, g, b = r+int(c.R), g+int(c.G), b+int(c.B)
		}
	}
	return fmt.Sprintf("#%02x%02x%02x", r/64, g/64, b/64)
}

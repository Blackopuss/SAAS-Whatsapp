package images

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/gen2brain/avif"
	"github.com/gen2brain/webp"
)

func pngOf(w, h int, c color.Color) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func TestDecodeRejects(t *testing.T) {
	if _, err := Decode([]byte("<script>alert(1)</script>")); err == nil {
		t.Fatal("aceptó texto como foto")
	}
	if _, err := Decode([]byte("GIF89a....")); err == nil {
		t.Fatal("aceptó GIF")
	}
	if _, err := Decode(nil); err == nil {
		t.Fatal("aceptó archivo vacío")
	}
	defer func(old int) { maxPixels = old }(maxPixels)
	maxPixels = 100
	if _, err := Decode(pngOf(20, 20, color.White)); err == nil {
		t.Fatal("aceptó foto por encima del límite de píxeles")
	}
}

func TestRenditions(t *testing.T) {
	img, err := Decode(pngOf(1000, 1250, color.RGBA{200, 30, 90, 255}))
	if err != nil {
		t.Fatal(err)
	}
	rs, err := Renditions(img)
	if err != nil {
		t.Fatal(err)
	}
	// 400 y 800 en dos formatos, y 1000 (original, sin agrandar a 1200).
	want := []int{400, 400, 800, 800, 1000, 1000}
	if len(rs) != len(want) {
		t.Fatalf("renditions = %d, quiero %d", len(rs), len(want))
	}
	for i, r := range rs {
		var cfg image.Config
		var err error
		if r.Format == "webp" {
			cfg, err = webp.DecodeConfig(bytes.NewReader(r.Data))
		} else {
			cfg, err = avif.DecodeConfig(bytes.NewReader(r.Data))
		}
		if err != nil || cfg.Width != want[i] || cfg.Height != want[i]*5/4 {
			t.Errorf("%d: %s %dx%d err=%v, quiero ancho %d", i, r.Format, cfg.Width, cfg.Height, err, want[i])
		}
	}
	if got := DominantColor(img); got != "#c81e5a" {
		t.Errorf("DominantColor = %s", got)
	}
}

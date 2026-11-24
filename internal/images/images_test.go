package images

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

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
	// 400, 800 y 1000 (original, sin agrandar a 1200), todo en WebP.
	want := []int{400, 800, 1000}
	if len(rs) != len(want) {
		t.Fatalf("renditions = %d, quiero %d", len(rs), len(want))
	}
	for i, r := range rs {
		cfg, err := webp.DecodeConfig(bytes.NewReader(r.Data))
		if r.Format != "webp" || err != nil || cfg.Width != want[i] || cfg.Height != want[i]*5/4 {
			t.Errorf("%d: %s %dx%d err=%v, quiero ancho %d", i, r.Format, cfg.Width, cfg.Height, err, want[i])
		}
	}
	if got := DominantColor(img); got != "#c81e5a" {
		t.Errorf("DominantColor = %s", got)
	}
}

// Un JPEG válido con más marcadores de scan (FF DA) que maxScans se rechaza antes de decodificar.
func TestDecodeRejectsManyScans(t *testing.T) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewGray(image.Rect(0, 0, 16, 16)), nil); err != nil {
		t.Fatal(err)
	}
	ok := buf.Bytes()
	if _, err := Decode(ok); err != nil {
		t.Fatalf("rechazó un JPEG normal: %v", err)
	}
	// Segmento de comentario (FF FE) con muchos FF DA: el decodificador lo salta, el conteo no.
	payload := bytes.Repeat([]byte{0xFF, 0xDA}, maxScans)
	com := append([]byte{0xFF, 0xFE, byte((len(payload) + 2) >> 8), byte(len(payload) + 2)}, payload...)
	bad := append(append(append([]byte{}, ok[:2]...), com...), ok[2:]...)
	if _, _, err := image.Decode(bytes.NewReader(bad)); err != nil {
		t.Fatalf("el JPEG de prueba debe seguir siendo válido: %v", err)
	}
	if _, err := Decode(bad); err == nil {
		t.Fatal("aceptó un JPEG con más scans que el límite")
	}
}

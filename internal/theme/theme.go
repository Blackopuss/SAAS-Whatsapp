// Package theme garantiza que el color de la vendedora se lea bien en botones (plan, sección 3.3).
package theme

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	Default = "#c2185b" // rosa mexicano de Vitrina
	White   = "#ffffff"
	Ink     = "#1f1d26"
	minRate = 4.5
)

// Fit devuelve el color que se usará (ajustado si hacía falta) y el color del texto encima.
// adjusted indica que hubo que cambiarlo para pasar 4.5:1.
func Fit(hex string) (brand, ink string, adjusted bool, err error) {
	c, err := parse(hex)
	if err != nil {
		return "", "", false, err
	}
	cw, ci := contrast(c, rgb{1, 1, 1}), contrast(c, mustParse(Ink))
	if cw >= minRate || ci >= minRate {
		if cw >= ci {
			return format(c), White, false, nil
		}
		return format(c), Ink, false, nil
	}
	// Ni blanco ni tinta pasan: mover la luminosidad en OKLCH, conservando tono y saturación, hasta pasar.
	l, ch, h := toOKLCH(c)
	ink, step, target := White, -0.01, rgb{1, 1, 1}
	if ci > cw {
		ink, step, target = Ink, 0.01, mustParse(Ink)
	}
	for range 100 {
		l = math.Max(0, math.Min(1, l+step))
		c = fromOKLCH(l, ch, h)
		if contrast(c, target) >= minRate {
			break
		}
	}
	return format(c), ink, true, nil
}

// InkFor elige el texto para un color ya guardado (ya pasó por Fit).
func InkFor(hex string) string {
	c, err := parse(hex)
	if err != nil || contrast(c, rgb{1, 1, 1}) >= contrast(c, mustParse(Ink)) {
		return White
	}
	return Ink
}

func Valid(hex string) bool { _, err := parse(hex); return err == nil }

type rgb struct{ r, g, b float64 } // sRGB 0..1

func parse(s string) (rgb, error) {
	s = strings.TrimSpace(s)
	if len(s) != 7 || s[0] != '#' {
		return rgb{}, fmt.Errorf("color inválido %q", s)
	}
	v, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return rgb{}, fmt.Errorf("color inválido %q", s)
	}
	return rgb{float64(v>>16&255) / 255, float64(v>>8&255) / 255, float64(v&255) / 255}, nil
}

func mustParse(s string) rgb { c, _ := parse(s); return c }

func format(c rgb) string {
	b := func(x float64) int { return int(math.Round(math.Max(0, math.Min(1, x)) * 255)) }
	return fmt.Sprintf("#%02x%02x%02x", b(c.r), b(c.g), b(c.b))
}

func lin(x float64) float64 {
	if x <= 0.04045 {
		return x / 12.92
	}
	return math.Pow((x+0.055)/1.055, 2.4)
}

func unlin(x float64) float64 {
	if x <= 0.0031308 {
		return 12.92 * x
	}
	return 1.055*math.Pow(x, 1/2.4) - 0.055
}

func luminance(c rgb) float64 { return 0.2126*lin(c.r) + 0.7152*lin(c.g) + 0.0722*lin(c.b) }

// contrast es la razón WCAG 2.x.
func contrast(a, b rgb) float64 {
	la, lb := luminance(a), luminance(b)
	return (math.Max(la, lb) + 0.05) / (math.Min(la, lb) + 0.05)
}

// Conversión sRGB <-> OKLab (Björn Ottosson).
func toOKLCH(c rgb) (l, ch, h float64) {
	r, g, b := lin(c.r), lin(c.g), lin(c.b)
	lm := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
	m := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
	s := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)
	L := 0.2104542553*lm + 0.7936177850*m - 0.0040720468*s
	A := 1.9779984951*lm - 2.4285922050*m + 0.4505937099*s
	B := 0.0259040371*lm + 0.7827717662*m - 0.8086757660*s
	return L, math.Hypot(A, B), math.Atan2(B, A)
}

func fromOKLCH(L, ch, h float64) rgb {
	A, B := ch*math.Cos(h), ch*math.Sin(h)
	cube := func(x float64) float64 { return x * x * x }
	l := cube(L + 0.3963377774*A + 0.2158037573*B)
	m := cube(L - 0.1055613458*A - 0.0638541728*B)
	s := cube(L - 0.0894841775*A - 1.2914855480*B)
	r := 4.0767416621*l - 3.3077115913*m + 0.2309699292*s
	g := -1.2684380046*l + 2.6097574011*m - 0.3413193965*s
	b := -0.0041960863*l - 0.7034186147*m + 1.7076147010*s
	clamp := func(x float64) float64 { return math.Max(0, math.Min(1, unlin(math.Max(0, x)))) }
	return rgb{clamp(r), clamp(g), clamp(b)}
}

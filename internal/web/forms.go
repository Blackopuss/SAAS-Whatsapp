package web

import (
	"errors"
	"strconv"
	"strings"
)

const maxPriceCents = 100_000_000 // 1 millón de pesos

// parsePrice convierte "289", "289.5", "$1,289.50" a centavos. Formato es-MX: coma de miles, punto decimal.
func parsePrice(s string) (int32, error) {
	s = strings.NewReplacer("$", "", ",", "", " ", "").Replace(strings.TrimSpace(s))
	if s == "" {
		return 0, errors.New("escribe el precio")
	}
	whole, frac, hasDot := strings.Cut(s, ".")
	if whole == "" {
		whole = "0"
	}
	if len(frac) > 2 || (hasDot && frac == "") || !digits(whole) || !digits(frac) || len(whole) > 7 {
		return 0, errors.New("el precio debe ser un número como 289 o 289.50")
	}
	for len(frac) < 2 {
		frac += "0"
	}
	n, err := strconv.ParseInt(whole+frac, 10, 64)
	if err != nil || n > maxPriceCents {
		return 0, errors.New("el precio es demasiado alto")
	}
	return int32(n), nil // #nosec G115 -- acotado por maxPriceCents
}

func digits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// splitList separa "S, M ,L" en elementos limpios, sin repetidos ni "|" (se usa como separador de variante).
func splitList(s string, maxLen int) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(strings.ReplaceAll(p, "|", ""))
		if r := []rune(p); len(r) > maxLen {
			p = string(r[:maxLen])
		}
		if p != "" && !seen[strings.ToLower(p)] {
			seen[strings.ToLower(p)] = true
			out = append(out, p)
		}
	}
	return out
}

type variantKey struct{ Size, Color string }

func (k variantKey) String() string { return k.Size + "|" + k.Color }

// variantMatrix arma todas las combinaciones talla × color; si falta una lista, usa "" (talla o color único).
func variantMatrix(sizes, colors []string) []variantKey {
	if len(sizes) == 0 && len(colors) == 0 {
		return nil
	}
	if len(sizes) == 0 {
		sizes = []string{""}
	}
	if len(colors) == 0 {
		colors = []string{""}
	}
	out := make([]variantKey, 0, len(sizes)*len(colors))
	for _, c := range colors {
		for _, s := range sizes {
			out = append(out, variantKey{s, c})
		}
	}
	return out
}

func formatPrice(cents int32) string {
	s := strconv.FormatInt(int64(cents)/100, 10)
	if c := cents % 100; c != 0 {
		return s + "." + strconv.FormatInt(int64(c)+100, 10)[1:]
	}
	return s
}

// Unidades de venta. Las cantidades se guardan en milésimas: 1 pieza = 1000, 0.25 kg = 250.
var units = map[string]string{"pieza": "pza", "kg": "kg", "g": "g", "litro": "L", "metro": "m"}

const maxQty = 1_000_000_000 // un millón de unidades, cabe en int32 con margen

// parseQty convierte "2", "0.25", "1,5" a milésimas. Vacío = sin valor (ok=false, sin error).
func parseQty(s string) (milli int32, ok bool, err error) {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", ".")
	if s == "" {
		return 0, false, nil
	}
	whole, frac, hasDot := strings.Cut(s, ".")
	if whole == "" {
		whole = "0"
	}
	if len(frac) > 3 || (hasDot && frac == "") || !digits(whole) || !digits(frac) || len(whole) > 7 {
		return 0, false, errors.New("debe ser un número como 10 o 2.5")
	}
	for len(frac) < 3 {
		frac += "0"
	}
	n, err := strconv.ParseInt(whole+frac, 10, 64)
	if err != nil || n > maxQty {
		return 0, false, errors.New("es demasiado grande")
	}
	return int32(n), true, nil // #nosec G115 -- acotado por maxQty
}

// formatQty: 2500 -> "2.5", 1000 -> "1".
func formatQty(milli int32) string {
	s := strconv.FormatInt(int64(milli)/1000, 10)
	if f := milli % 1000; f != 0 {
		s += "." + strings.TrimRight(strconv.FormatInt(int64(f)+1000, 10)[1:], "0")
	}
	return s
}

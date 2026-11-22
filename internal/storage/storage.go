// Package storage guarda las fotos ya procesadas.
package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type Storage interface {
	Put(ctx context.Context, key string, data []byte, contentType string) error
	Delete(ctx context.Context, keys ...string) error
}

// NewBaseKey da una llave aleatoria por tienda; nunca usa el nombre original del archivo.
func NewBaseKey(storeID int64) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("stores/%d/%s", storeID, hex.EncodeToString(b))
}

func VariantKey(base string, width int32, format string) string {
	return fmt.Sprintf("%s-%d.%s", base, width, format)
}

// Disk guarda en una carpeta local que el servidor publica en /media/.
// shortcut: solo para desarrollo; en producción va R2 (sprint 6) con la misma interfaz.
type Disk struct{ Dir string }

func (d Disk) path(key string) (string, error) {
	if !strings.HasPrefix(key, "stores/") || strings.Contains(key, "..") {
		return "", errors.New("llave inválida")
	}
	return filepath.Join(d.Dir, filepath.FromSlash(key)), nil
}

func (d Disk) Put(_ context.Context, key string, data []byte, _ string) error {
	p, err := d.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

func (d Disk) Delete(_ context.Context, keys ...string) error {
	var errs []error
	for _, k := range keys {
		p, err := d.path(k)
		if err == nil {
			err = os.Remove(p)
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

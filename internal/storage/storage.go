// Package storage guarda las fotos ya procesadas.
package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
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

// R2 guarda en Cloudflare R2 por su API compatible con S3.
type R2 struct {
	c      *minio.Client
	bucket string
}

func NewR2(accountID, accessKey, secretKey, bucket string) (*R2, error) {
	c, err := minio.New(accountID+".r2.cloudflarestorage.com", &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: true,
		Region: "auto",
	})
	if err != nil {
		return nil, err
	}
	return &R2{c: c, bucket: bucket}, nil
}

func (r *R2) Put(ctx context.Context, key string, data []byte, contentType string) error {
	_, err := r.c.PutObject(ctx, r.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType:  contentType,
		CacheControl: "public, max-age=31536000, immutable", // llaves aleatorias: el contenido nunca cambia
	})
	return err
}

func (r *R2) Delete(ctx context.Context, keys ...string) error {
	var errs []error
	for _, k := range keys {
		if err := r.c.RemoveObject(ctx, r.bucket, k, minio.RemoveObjectOptions{}); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

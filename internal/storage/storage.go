package storage

import (
	"context"
	"io"
	"os"
)

type PutResult struct{ ETag string }

type Store interface {
	Ready(ctx context.Context) error
	Key(parts ...string) string
	Put(ctx context.Context, key, contentType string, size int64, body io.Reader) (PutResult, error)
	Open(ctx context.Context, key string) (*os.File, os.FileInfo, error)
	Delete(ctx context.Context, key string) error
}

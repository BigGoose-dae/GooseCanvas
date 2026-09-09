package storage

import (
	"context"
	"io"
	"time"
)

type PutResult struct{ ETag string }

type Store interface {
	Ready(ctx context.Context) error
	Put(ctx context.Context, key, contentType string, size int64, body io.Reader) (PutResult, error)
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	Delete(ctx context.Context, key string) error
}

package storage

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/BigGoose-dae/GooseCanvas/internal/config"
	"github.com/volcengine/ve-tos-golang-sdk/v2/tos"
	"github.com/volcengine/ve-tos-golang-sdk/v2/tos/enum"
)

type TOSStore struct {
	client *tos.ClientV2
	bucket string
	prefix string
	ttl    time.Duration
}

func NewTOS(cfg config.TOS) (*TOSStore, error) {
	if cfg.Endpoint == "" || cfg.Region == "" || cfg.Bucket == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("TOS configuration is incomplete")
	}
	client, err := tos.NewClientV2(cfg.Endpoint,
		tos.WithRegion(cfg.Region),
		tos.WithCredentials(tos.NewStaticCredentials(cfg.AccessKey, cfg.SecretKey)),
		tos.WithMaxRetryCount(3),
	)
	if err != nil {
		return nil, fmt.Errorf("create TOS client: %w", err)
	}
	return &TOSStore{client: client, bucket: cfg.Bucket, prefix: strings.Trim(cfg.Prefix, "/"), ttl: cfg.PresignTTL}, nil
}

func (s *TOSStore) Key(parts ...string) string {
	items := append([]string{s.prefix}, parts...)
	return path.Join(items...)
}

func (s *TOSStore) Bucket() string { return s.bucket }

func (s *TOSStore) Ready(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &tos.HeadBucketInput{Bucket: s.bucket})
	if err != nil {
		return fmt.Errorf("access TOS bucket %q: %w", s.bucket, err)
	}
	return nil
}

func (s *TOSStore) Put(ctx context.Context, key, contentType string, size int64, body io.Reader) (PutResult, error) {
	out, err := s.client.PutObjectV2(ctx, &tos.PutObjectV2Input{
		PutObjectBasicInput: tos.PutObjectBasicInput{Bucket: s.bucket, Key: key, ContentLength: size, ContentType: contentType},
		Content:             body,
	})
	if err != nil {
		return PutResult{}, fmt.Errorf("put TOS object: %w", err)
	}
	return PutResult{ETag: strings.Trim(out.ETag, "\"")}, nil
}

func (s *TOSStore) PresignGet(_ context.Context, key string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = s.ttl
	}
	if ttl > 7*24*time.Hour {
		ttl = 7 * 24 * time.Hour
	}
	out, err := s.client.PreSignedURL(&tos.PreSignedURLInput{HTTPMethod: enum.HttpMethodGet, Bucket: s.bucket, Key: key, Expires: int64(ttl.Seconds())})
	if err != nil {
		return "", fmt.Errorf("presign TOS object: %w", err)
	}
	return out.SignedUrl, nil
}

func (s *TOSStore) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObjectV2(ctx, &tos.DeleteObjectV2Input{Bucket: s.bucket, Key: key})
	return err
}

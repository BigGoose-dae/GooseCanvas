package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type LocalStore struct {
	root string
}

func NewLocal(root string) (*LocalStore, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve local asset directory: %w", err)
	}
	store := &LocalStore{root: absolute}
	if err := store.Ready(context.Background()); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *LocalStore) Root() string { return s.root }

func (s *LocalStore) Ready(_ context.Context) error {
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return fmt.Errorf("create local asset directory: %w", err)
	}
	if err := os.Chmod(s.root, 0o700); err != nil {
		return fmt.Errorf("protect local asset directory: %w", err)
	}
	return nil
}

func (s *LocalStore) Key(parts ...string) string {
	return path.Join(parts...)
}

func (s *LocalStore) resolve(key string) (string, error) {
	trimmed := strings.TrimSpace(key)
	clean := path.Clean(trimmed)
	localKey := filepath.FromSlash(clean)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) || filepath.IsAbs(localKey) || strings.ContainsRune(key, '\x00') {
		return "", fmt.Errorf("invalid local asset key")
	}
	full := filepath.Join(s.root, localKey)
	relative, err := filepath.Rel(s.root, full)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("local asset key escapes storage directory")
	}
	return full, nil
}

func (s *LocalStore) Put(ctx context.Context, key, _ string, size int64, body io.Reader) (PutResult, error) {
	destination, err := s.resolve(key)
	if err != nil {
		return PutResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return PutResult{}, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(destination), ".goose-upload-*")
	if err != nil {
		return PutResult{}, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return PutResult{}, err
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(tmp, hash), &contextReader{ctx: ctx, reader: body})
	if copyErr != nil {
		tmp.Close()
		return PutResult{}, copyErr
	}
	if size >= 0 && written != size {
		tmp.Close()
		return PutResult{}, fmt.Errorf("local asset size mismatch: expected %d bytes, wrote %d", size, written)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return PutResult{}, err
	}
	if err := tmp.Close(); err != nil {
		return PutResult{}, err
	}
	if err := os.Rename(tmpName, destination); err != nil {
		return PutResult{}, err
	}
	return PutResult{ETag: hex.EncodeToString(hash.Sum(nil))}, nil
}

func (s *LocalStore) Open(_ context.Context, key string) (*os.File, os.FileInfo, error) {
	name, err := s.resolve(key)
	if err != nil {
		return nil, nil, err
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		file.Close()
		return nil, nil, fmt.Errorf("local asset is not a regular file")
	}
	return file, info, nil
}

func (s *LocalStore) Delete(_ context.Context, key string) error {
	name, err := s.resolve(key)
	if err != nil {
		return err
	}
	err = os.Remove(name)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(buffer []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.reader.Read(buffer)
	}
}

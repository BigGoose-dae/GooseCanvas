package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"
)

func TestLocalStorePutOpenAndDelete(t *testing.T) {
	store, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("local asset")
	result, err := store.Put(context.Background(), store.Key("uploads", "asset.bin"), "application/octet-stream", int64(len(data)), bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	wantHash := sha256.Sum256(data)
	if result.ETag != hex.EncodeToString(wantHash[:]) {
		t.Fatalf("etag = %q", result.ETag)
	}
	file, info, err := store.Open(context.Background(), "uploads/asset.bin")
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := io.ReadAll(file)
	file.Close()
	if err != nil || !bytes.Equal(loaded, data) || info.Size() != int64(len(data)) {
		t.Fatalf("loaded asset = %q, size = %d, err = %v", loaded, info.Size(), err)
	}
	if err := store.Delete(context.Background(), "uploads/asset.bin"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Open(context.Background(), "uploads/asset.bin"); err == nil {
		t.Fatal("deleted asset remained readable")
	}
}

func TestLocalStoreRejectsInvalidKeysAndSizes(t *testing.T) {
	store, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(context.Background(), "", "text/plain", 1, bytes.NewReader([]byte("x"))); err == nil {
		t.Fatal("empty key was accepted")
	}
	if _, err := store.Put(context.Background(), "../outside", "text/plain", 1, bytes.NewReader([]byte("x"))); err == nil {
		t.Fatal("traversal key was accepted")
	}
	if _, err := store.Put(context.Background(), "short", "text/plain", 2, bytes.NewReader([]byte("x"))); err == nil {
		t.Fatal("incorrect size was accepted")
	}
}

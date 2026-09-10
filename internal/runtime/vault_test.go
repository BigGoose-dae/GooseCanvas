package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVaultEncryptsAndAuthenticatesSettings(t *testing.T) {
	dir := t.TempDir()
	original := []byte(`{"credential":"fixture-not-a-real-credential"}`)
	encrypted, err := sealSettings(dir, original)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encrypted, "fixture-not-a-real-credential") {
		t.Fatal("plaintext in ciphertext")
	}
	decoded, err := openSettings(dir, encrypted)
	if err != nil || string(decoded) != string(original) {
		t.Fatal("cannot decrypt saved configuration")
	}
	info, err := os.Stat(filepath.Join(dir, ".settings.key"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal("key file permissions are too broad")
	}
	// An unrelated key cannot authenticate or decrypt the record.
	other := t.TempDir()
	if _, err := sealSettings(other, original); err != nil {
		t.Fatal(err)
	}
	if _, err := openSettings(other, encrypted); err == nil {
		t.Fatal("wrong key accepted")
	}
}

func TestVaultNeverReplacesMissingKeyForEncryptedSettings(t *testing.T) {
	dir := t.TempDir()
	if _, err := openSettings(dir, vaultPrefix+"test"); err == nil {
		t.Fatal("missing key accepted")
	}
	if _, err := os.Stat(filepath.Join(dir, ".settings.key")); !os.IsNotExist(err) {
		t.Fatal("missing key silently replaced")
	}
}

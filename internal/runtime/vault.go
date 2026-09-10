package runtime

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const vaultPrefix = "aesgcm:v1:"

// The encryption key is local-only, stored separately from database backups.
// Losing it makes saved credentials unrecoverable; never silently replace it.
func vaultKey(dir string, create bool) ([]byte, error) {
	path := filepath.Join(dir, ".settings.key")
	key, err := os.ReadFile(path)
	if err == nil {
		if len(key) != 32 {
			return nil, fmt.Errorf("本机配置加密密钥损坏")
		}
		if err := os.Chmod(path, 0o600); err != nil {
			return nil, err
		}
		return key, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	if !create {
		return nil, fmt.Errorf("缺少本机配置加密密钥，请恢复 data/.settings.key 备份")
	}
	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if os.IsExist(err) {
		return vaultKey(dir, false)
	}
	if err != nil {
		return nil, err
	}
	if _, err := file.Write(key); err != nil {
		file.Close()
		return nil, err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	return key, nil
}

func sealSettings(dir string, plaintext []byte) (string, error) {
	key, err := vaultKey(dir, true)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ciphertext := aead.Seal(nonce, nonce, plaintext, []byte(vaultPrefix))
	return vaultPrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

func openSettings(dir, value string) ([]byte, error) {
	key, err := vaultKey(dir, false)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, vaultPrefix))
	if err != nil || len(raw) < aead.NonceSize() {
		return nil, fmt.Errorf("加密配置格式无效")
	}
	nonce, ciphertext := raw[:aead.NonceSize()], raw[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, ciphertext, []byte(vaultPrefix))
	if err != nil {
		return nil, fmt.Errorf("配置解密失败，请检查本机加密密钥")
	}
	return plaintext, nil
}

package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// encPrefix marks an AES-GCM encrypted value in cameras.yaml. Values without
// it are treated as legacy plaintext (read transparently, re-encrypted on next
// save), so upgrades are seamless.
const encPrefix = "enc:"

var (
	aeadOnce sync.Once
	aead     cipher.AEAD
	aeadErr  error
)

// keyPath is where the AES key lives. Overridable via KSPCAM_KEY_FILE; defaults
// to ~/.kspcam.key. The deploy points this at /opt/ksp-cam/.kspcam.key.
func keyPath() string {
	if p := os.Getenv("KSPCAM_KEY_FILE"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".kspcam.key"
	}
	return filepath.Join(home, ".kspcam.key")
}

// loadOrCreateKey returns a 32-byte key. KSPCAM_KEY (base64 32 bytes, or any
// string hashed to 32 bytes) wins; otherwise a key file is read or generated.
func loadOrCreateKey() ([]byte, error) {
	if k := os.Getenv("KSPCAM_KEY"); k != "" {
		if b, err := base64.StdEncoding.DecodeString(k); err == nil && len(b) == 32 {
			return b, nil
		}
		h := sha256.Sum256([]byte(k))
		return h[:], nil
	}
	p := keyPath()
	if b, err := os.ReadFile(p); err == nil && len(b) >= 32 {
		return b[:32], nil
	}
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	if err := os.WriteFile(p, key, 0o600); err != nil {
		return nil, fmt.Errorf("write key %s: %w", p, err)
	}
	return key, nil
}

func gcm() (cipher.AEAD, error) {
	aeadOnce.Do(func() {
		key, err := loadOrCreateKey()
		if err != nil {
			aeadErr = err
			return
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			aeadErr = err
			return
		}
		aead, aeadErr = cipher.NewGCM(block)
	})
	return aead, aeadErr
}

// Encrypt returns an "enc:<base64>" AES-256-GCM ciphertext of plain (empty in →
// empty out).
func Encrypt(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	if strings.HasPrefix(plain, encPrefix) { // already encrypted
		return plain, nil
	}
	a, err := gcm()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, a.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := a.Seal(nonce, nonce, []byte(plain), nil)
	return encPrefix + base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt reverses Encrypt. A value without the enc: prefix is returned as-is
// (legacy plaintext).
func Decrypt(s string) (string, error) {
	if !strings.HasPrefix(s, encPrefix) {
		return s, nil
	}
	a, err := gcm()
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(s, encPrefix))
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	ns := a.NonceSize()
	if len(raw) < ns {
		return "", errors.New("ciphertext too short")
	}
	pt, err := a.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(pt), nil
}

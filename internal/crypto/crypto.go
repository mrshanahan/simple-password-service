package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

const (
	KeySize int = 32
)

type PassdKey struct {
	key []byte
}

func NewPassdKey(key []byte) (PassdKey, error) {
	if len(key) != KeySize {
		return PassdKey{}, fmt.Errorf("invalid key size (got %d, expected %d)", len(key), KeySize)
	}
	return PassdKey{key}, nil
}

func GeneratePassdKey() (PassdKey, error) {
	key := make([]byte, KeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return PassdKey{}, fmt.Errorf("failed to populate key from rand: %w", err)
	}
	return PassdKey{key}, nil
}

func Load(keyFile *os.File) (PassdKey, error) {
	keyData, err := io.ReadAll(keyFile)
	if err != nil {
		return PassdKey{}, fmt.Errorf("failed to read key file: %w", err)
	}

	return NewPassdKey(keyData)
}

func (k *PassdKey) Save(path string) error {
	if err := os.WriteFile(path, k.key, 0o600); err != nil {
		return fmt.Errorf("failed to write key: %w", err)
	}
	return nil
}

func (k *PassdKey) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(k.key)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cipher block: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap cipher in GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate GCM nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	return ciphertext, nil
}

func (k *PassdKey) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(k.key)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cipher block: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap cipher in GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("invalid ciphertext - text length is less than nonce size (%d < %d)", len(ciphertext), nonceSize)
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt ciphertext: %w", err)
	}

	return plaintext, nil
}

func Hash(data []byte) ([]byte, error) {
	hash := sha256.New()
	n, err := hash.Write(data)
	if err != nil {
		return nil, fmt.Errorf("failed to hash bytes (%d/%d bytes): %w", n, len(data), err)
	}
	return hash.Sum(nil), nil
}

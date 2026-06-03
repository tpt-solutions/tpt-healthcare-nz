// Package encryption provides AES-256-GCM field-level encryption for sensitive health data.
package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

const keySize = 32 // 256 bits

// Encryptor performs AES-256-GCM encryption and decryption.
type Encryptor struct {
	key []byte
}

// New creates an Encryptor from the given key. The key must be exactly 32 bytes.
func New(key []byte) (*Encryptor, error) {
	if len(key) != keySize {
		return nil, fmt.Errorf("encryption: key must be %d bytes, got %d", keySize, len(key))
	}
	k := make([]byte, keySize)
	copy(k, key)
	return &Encryptor{key: k}, nil
}

// NewFromEnv creates an Encryptor by reading the ENCRYPTION_KEY environment variable.
// The variable must be a hex-encoded 32-byte (64 hex-character) key.
func NewFromEnv() (*Encryptor, error) {
	val := os.Getenv("ENCRYPTION_KEY")
	if val == "" {
		return nil, errors.New("encryption: ENCRYPTION_KEY environment variable is not set")
	}
	key, err := hex.DecodeString(val)
	if err != nil {
		return nil, fmt.Errorf("encryption: failed to hex-decode ENCRYPTION_KEY: %w", err)
	}
	return New(key)
}

// Encrypt encrypts plaintext using AES-256-GCM. It prepends a random 12-byte nonce to
// the output, so the returned bytes are: nonce (12) || ciphertext || GCM tag (16).
func (e *Encryptor) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("encryption: failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("encryption: failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("encryption: failed to generate nonce: %w", err)
	}

	// Seal appends ciphertext+tag to nonce so the layout is nonce||ciphertext||tag.
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext produced by Encrypt. It expects the nonce to be prepended
// to the ciphertext (i.e. the format returned by Encrypt).
func (e *Encryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("encryption: failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("encryption: failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("encryption: ciphertext too short")
	}

	nonce, ciphertextBody := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBody, nil)
	if err != nil {
		return nil, fmt.Errorf("encryption: decryption failed: %w", err)
	}
	return plaintext, nil
}

// EncryptString encrypts a string and returns the result as a base64 standard-encoded string.
func (e *Encryptor) EncryptString(s string) (string, error) {
	enc, err := e.Encrypt([]byte(s))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(enc), nil
}

// DecryptString base64-decodes the input and then decrypts it, returning the plaintext string.
func (e *Encryptor) DecryptString(s string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", fmt.Errorf("encryption: failed to base64-decode input: %w", err)
	}
	plaintext, err := e.Decrypt(data)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

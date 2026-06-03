package encryption

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testKey is a 32-byte key used across tests.
var testKey = bytes.Repeat([]byte{0x42}, 32)

func newTestEncryptor(t *testing.T) *Encryptor {
	t.Helper()
	e, err := New(testKey)
	require.NoError(t, err)
	return e
}

func TestEncryptDecrypt(t *testing.T) {
	e := newTestEncryptor(t)

	plaintext := []byte("sensitive patient data: NHI ZAC5361")

	ciphertext, err := e.Encrypt(plaintext)
	require.NoError(t, err, "Encrypt should not return an error")
	assert.NotEqual(t, plaintext, ciphertext, "ciphertext should differ from plaintext")

	recovered, err := e.Decrypt(ciphertext)
	require.NoError(t, err, "Decrypt should not return an error")
	assert.Equal(t, plaintext, recovered, "decrypted value should match original plaintext")
}

func TestEncryptString(t *testing.T) {
	e := newTestEncryptor(t)

	original := "He tāngata, he tāngata, he tāngata"

	encoded, err := e.EncryptString(original)
	require.NoError(t, err, "EncryptString should not return an error")
	assert.NotEmpty(t, encoded, "EncryptString should return a non-empty base64 string")
	assert.NotEqual(t, original, encoded, "encoded value should differ from plaintext")

	decoded, err := e.DecryptString(encoded)
	require.NoError(t, err, "DecryptString should not return an error")
	assert.Equal(t, original, decoded, "round-tripped string should match original")
}

func TestNewInvalidKey(t *testing.T) {
	tests := []struct {
		name string
		key  []byte
	}{
		{name: "key too short (16 bytes)", key: bytes.Repeat([]byte{0x01}, 16)},
		{name: "key too long (64 bytes)", key: bytes.Repeat([]byte{0x01}, 64)},
		{name: "empty key", key: []byte{}},
		{name: "nil key", key: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc, err := New(tt.key)
			assert.Nil(t, enc, "New should return nil Encryptor for invalid key length")
			assert.Error(t, err, "New should return an error for invalid key length")
		})
	}
}

func TestEncryptProducesUniqueNonce(t *testing.T) {
	e := newTestEncryptor(t)

	plaintext := []byte("same plaintext encrypted twice")

	ct1, err := e.Encrypt(plaintext)
	require.NoError(t, err)

	ct2, err := e.Encrypt(plaintext)
	require.NoError(t, err)

	assert.NotEqual(t, ct1, ct2,
		"two encryptions of the same plaintext must produce different ciphertexts due to random nonce")

	// Both ciphertexts must decrypt back to the same plaintext.
	pt1, err := e.Decrypt(ct1)
	require.NoError(t, err)
	assert.Equal(t, plaintext, pt1)

	pt2, err := e.Decrypt(ct2)
	require.NoError(t, err)
	assert.Equal(t, plaintext, pt2)
}

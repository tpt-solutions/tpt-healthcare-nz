package testutil

import (
	"context"
	"crypto/ed25519"
	crand "crypto/rand"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/google/uuid"
)

// TestContext returns a background context with a 30-second timeout.
func TestContext() context.Context {
	ctx, _ := context.WithTimeout(context.Background(), 30*time.Second)
	return ctx
}

// RandomUUID returns a new random UUID.
func RandomUUID() uuid.UUID {
	return uuid.New()
}

// RandomNHI generates a valid old-format NHI: 3 letters + 4 digits with Luhn checksum.
func RandomNHI() string {
	const letters = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	const digits = "0123456789"

	b := make([]byte, 7)
	for i := 0; i < 3; i++ {
		b[i] = letters[rand.IntN(len(letters))]
	}
	for i := 3; i < 7; i++ {
		b[i] = digits[rand.IntN(len(digits))]
	}

	total := 0
	for i := 0; i < 6; i++ {
		d := int(b[i] - '0')
		if i%2 == 0 {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		total += d
	}
	checkDigit := (10 - (total % 10)) % 10
	b[6] = byte('0' + checkDigit)

	return string(b)
}

// RandomCPN generates a random HPI CPN (10-digit number).
func RandomCPN() string {
	return fmt.Sprintf("%010d", rand.IntN(10000000000))
}

// GenerateEd25519Key generates a new Ed25519 key pair for JWT tests.
func GenerateEd25519Key() (ed25519.PublicKey, ed25519.PrivateKey) {
	pub, priv, _ := ed25519.GenerateKey(crand.Reader)
	return pub, priv
}

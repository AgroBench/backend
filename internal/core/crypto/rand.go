package crypto

import (
	"crypto/rand"
	"encoding/hex"
)

func randRead(b []byte) (int, error) { return rand.Read(b) }

// RandomHex devolve n bytes aleatórios em hex (2n caracteres). Usado em refresh tokens e nonces.
func RandomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

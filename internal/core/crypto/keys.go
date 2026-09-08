package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// ParseKey32 decodifica uma chave de 32 bytes em hex.
func ParseKey32(hexKey string) (*[KeySize]byte, error) {
	raw, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("chave não é hex: %w", err)
	}
	if len(raw) != KeySize {
		return nil, fmt.Errorf("chave deve ter %d bytes, tem %d", KeySize, len(raw))
	}
	var k [KeySize]byte
	copy(k[:], raw)
	return &k, nil
}

func EncodeKey(k *[KeySize]byte) string { return hex.EncodeToString(k[:]) }

// GenerateSigningKey gera um par ed25519 para assinar vereditos.
func GenerateSigningKey() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(rand.Reader)
}

// ParseSigningSeed reconstrói a chave ed25519 a partir do seed de 32 bytes em hex.
func ParseSigningSeed(hexSeed string) (ed25519.PrivateKey, error) {
	seed, err := ParseKey32(hexSeed)
	if err != nil {
		return nil, err
	}
	return ed25519.NewKeyFromSeed(seed[:]), nil
}

package mock

import (
	"golang.org/x/crypto/curve25519"

	"github.com/AgroBench/backend/internal/core/crypto"
)

func publicFromPrivate(priv *[crypto.KeySize]byte) *[crypto.KeySize]byte {
	var pub [crypto.KeySize]byte
	curve25519.ScalarBaseMult(&pub, priv)
	return &pub
}

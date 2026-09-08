package auth

import (
	"github.com/spf13/viper"

	"github.com/AgroBench/backend/internal/core/crypto"
)

// HMACIdentifier aplica HMAC-SHA256(pepper, value) em hex. O pepper vem de AUTH_PEPPER
// (nunca do json de config). Usado para CPF, CAR e CNPJ (README §2, §10).
func HMACIdentifier(value string) string {
	return crypto.HMACHex(viper.GetString("auth.pepper"), value)
}

func Pepper() string { return viper.GetString("auth.pepper") }

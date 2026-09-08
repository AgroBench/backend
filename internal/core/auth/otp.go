package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"math/big"

	"github.com/AgroBench/backend/internal/core/crypto"
)

// GenerateOTP devolve um código de 6 dígitos e o sha256 hex para persistir.
func GenerateOTP() (plain, hash string, err error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", "", err
	}
	plain = fmt.Sprintf("%06d", n.Int64())
	return plain, HashOTP(plain), nil
}

func HashOTP(plain string) string { return crypto.SHA256Hex([]byte(plain)) }

// OTPMatch compara o código informado com o hash persistido, em tempo constante.
func OTPMatch(plain, hash string) bool {
	got := HashOTP(plain)
	if len(got) != len(hash) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(hash)) == 1
}

func OTPMessage(code string) string {
	return fmt.Sprintf("AgroBench: seu código é %s. Válido por 5 minutos.", code)
}

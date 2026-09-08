package auth

import (
	"time"

	"github.com/spf13/viper"

	"github.com/AgroBench/backend/internal/core/crypto"
)

// NewRefreshToken gera 32 bytes aleatórios (hex) e o sha256 para persistir (README §10).
func NewRefreshToken() (plain, hash string, err error) {
	plain, err = crypto.RandomHex(32)
	if err != nil {
		return "", "", err
	}
	return plain, HashRefresh(plain), nil
}

func HashRefresh(plain string) string { return crypto.SHA256Hex([]byte(plain)) }

func RefreshTTL() time.Duration {
	d := viper.GetInt("auth.refresh_ttl_days")
	if d <= 0 {
		d = 30
	}
	return time.Duration(d) * 24 * time.Hour
}

func OTPTL() time.Duration {
	m := viper.GetInt("auth.otp_ttl_minutes")
	if m <= 0 {
		m = 5
	}
	return time.Duration(m) * time.Minute
}

func OTPMaxAttempts() int {
	n := viper.GetInt("auth.otp_max_attempts")
	if n <= 0 {
		n = 5
	}
	return n
}

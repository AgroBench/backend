package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/spf13/viper"

	"github.com/AgroBench/backend/internal/core/domain"
)

const (
	PurposeAccess = "access"
	PurposeMFA    = "mfa"
)

var (
	ErrTokenInvalid = errors.New("token inválido")
	ErrTokenExpired = errors.New("token expirado")
	ErrWrongPurpose = errors.New("token com propósito incorreto")
)

// Claims do access JWT: sub, role, wallet (pubkey, pode ser vazio), jti (README §10).
type Claims struct {
	Role    string `json:"role"`
	Wallet  string `json:"wallet,omitempty"`
	Purpose string `json:"purpose"`
	jwt.RegisteredClaims
}

func (c *Claims) UserID() uuid.UUID {
	id, _ := uuid.Parse(c.Subject)
	return id
}

func (c *Claims) RoleValue() domain.Role { return domain.Role(c.Role) }

func IssueAccess(userID uuid.UUID, role domain.Role, wallet string) (string, error) {
	ttl := time.Duration(viper.GetInt("auth.access_ttl_minutes")) * time.Minute
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return issue(userID, role, wallet, PurposeAccess, ttl)
}

func IssueMFA(userID uuid.UUID) (string, error) {
	ttl := time.Duration(viper.GetInt("auth.otp_ttl_minutes")) * time.Minute
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return issue(userID, "", "", PurposeMFA, ttl)
}

func issue(userID uuid.UUID, role domain.Role, wallet, purpose string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		Role:    string(role),
		Wallet:  wallet,
		Purpose: purpose,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ID:        domain.NewID().String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Issuer:    "agrobench",
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString(jwtSecret())
	if err != nil {
		return "", fmt.Errorf("assinando jwt: %w", err)
	}
	return signed, nil
}

func ParseAccess(token string) (*Claims, error) {
	c, err := parse(token)
	if err != nil {
		return nil, err
	}
	if c.Purpose != PurposeAccess {
		return nil, ErrWrongPurpose
	}
	return c, nil
}

func ParseMFA(token string) (*Claims, error) {
	c, err := parse(token)
	if err != nil {
		return nil, err
	}
	if c.Purpose != PurposeMFA {
		return nil, ErrWrongPurpose
	}
	return c, nil
}

func parse(token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("alg inesperado: %s", t.Method.Alg())
		}
		return jwtSecret(), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, fmt.Errorf("%w: %v", ErrTokenInvalid, err)
	}
	c, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, ErrTokenInvalid
	}
	return c, nil
}

func jwtSecret() []byte {
	return []byte(viper.GetString("auth.jwt_secret"))
}

func AccessTTLSeconds() int {
	m := viper.GetInt("auth.access_ttl_minutes")
	if m <= 0 {
		m = 15
	}
	return m * 60
}

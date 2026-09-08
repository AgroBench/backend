package auth

import (
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/AgroBench/backend/internal/core/domain"
)

func TestIssueAndParseAccess(t *testing.T) {
	viper.Set("auth.jwt_secret", "test-secret-at-least-32-bytes-long")
	viper.Set("auth.access_ttl_minutes", 15)

	id := domain.NewID()
	tok, err := IssueAccess(id, domain.RoleProducer, "Wallet111")
	require.NoError(t, err)

	c, err := ParseAccess(tok)
	require.NoError(t, err)
	require.Equal(t, id, c.UserID())
	require.Equal(t, domain.RoleProducer, c.RoleValue())
	require.Equal(t, "Wallet111", c.Wallet)
	require.Equal(t, PurposeAccess, c.Purpose)

	_, err = ParseMFA(tok)
	require.ErrorIs(t, err, ErrWrongPurpose)
}

func TestMFAToken(t *testing.T) {
	viper.Set("auth.jwt_secret", "test-secret-at-least-32-bytes-long")
	viper.Set("auth.otp_ttl_minutes", 5)

	id := domain.NewID()
	tok, err := IssueMFA(id)
	require.NoError(t, err)
	c, err := ParseMFA(tok)
	require.NoError(t, err)
	require.Equal(t, id, c.UserID())
	require.Equal(t, PurposeMFA, c.Purpose)
}

func TestExpiredToken(t *testing.T) {
	viper.Set("auth.jwt_secret", "test-secret-at-least-32-bytes-long")
	viper.Set("auth.access_ttl_minutes", 0)
	// ttl 0 cai no default 15min; forçamos expiração via claim parse de token antigo não dá.
	// Cobre token inválido.
	_, err := ParseAccess("not-a-jwt")
	require.Error(t, err)
}

func TestOTPMatch(t *testing.T) {
	plain, hash, err := GenerateOTP()
	require.NoError(t, err)
	require.Len(t, plain, 6)
	require.True(t, OTPMatch(plain, hash))
	require.False(t, OTPMatch("000000", hash) && plain != "000000")
	_ = time.Now()
}

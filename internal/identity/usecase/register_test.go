package usecase_test

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/AgroBench/backend/internal/apperrors"
	"github.com/AgroBench/backend/internal/core/auth"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/internal/identity/domain"
	"github.com/AgroBench/backend/internal/identity/types/input"
	"github.com/AgroBench/backend/internal/identity/usecase"
	smsmock "github.com/AgroBench/backend/pkg/adapter/sms/mock"
)

func initAuthCfg() {
	viper.Set("auth.pepper", "test-pepper")
	viper.Set("auth.jwt_secret", "test-secret-at-least-32-bytes-long!!")
	viper.Set("auth.access_ttl_minutes", 15)
	viper.Set("auth.refresh_ttl_days", 30)
	viper.Set("auth.otp_ttl_minutes", 5)
	viper.Set("auth.otp_max_attempts", 5)
}

func nf() error { return apperrors.NotFound("mem", errors.New("not found")) }

type memUsers struct {
	byID    map[uuid.UUID]domain.User
	byEmail map[string]domain.User
	byCPF   map[string]domain.User
}

func newMemUsers() *memUsers {
	return &memUsers{
		byID:    map[uuid.UUID]domain.User{},
		byEmail: map[string]domain.User{},
		byCPF:   map[string]domain.User{},
	}
}

func (m *memUsers) Create(_ context.Context, u domain.User) error {
	m.byID[u.ID] = u
	m.byEmail[u.Email] = u
	m.byCPF[u.CPFHMAC] = u
	return nil
}
func (m *memUsers) UpdatePassword(_ context.Context, id uuid.UUID, hash string) error {
	u := m.byID[id]
	u.PasswordHash = hash
	m.byID[id] = u
	return nil
}
func (m *memUsers) GetByID(_ context.Context, id uuid.UUID) (domain.User, error) {
	u, ok := m.byID[id]
	if !ok {
		return domain.User{}, nf()
	}
	return u, nil
}
func (m *memUsers) GetByEmail(_ context.Context, email string) (domain.User, error) {
	u, ok := m.byEmail[email]
	if !ok {
		return domain.User{}, nf()
	}
	return u, nil
}
func (m *memUsers) GetByCPFHMAC(_ context.Context, hmac string) (domain.User, error) {
	u, ok := m.byCPF[hmac]
	if !ok {
		return domain.User{}, nf()
	}
	return u, nil
}

type memOTPs struct{ items []domain.OTPCode }

func (m *memOTPs) Create(_ context.Context, o domain.OTPCode) error {
	m.items = append(m.items, o)
	return nil
}
func (m *memOTPs) InvalidatePending(_ context.Context, userID uuid.UUID, purpose domain.OTPPurpose) error {
	now := time.Now()
	for i := range m.items {
		if m.items[i].UserID == userID && m.items[i].Purpose == purpose && m.items[i].ConsumedAt == nil {
			m.items[i].ConsumedAt = &now
		}
	}
	return nil
}
func (m *memOTPs) IncrementAttempts(_ context.Context, id uuid.UUID) error {
	for i := range m.items {
		if m.items[i].ID == id {
			m.items[i].Attempts++
		}
	}
	return nil
}
func (m *memOTPs) Consume(_ context.Context, id uuid.UUID) error {
	now := time.Now()
	for i := range m.items {
		if m.items[i].ID == id {
			m.items[i].ConsumedAt = &now
		}
	}
	return nil
}
func (m *memOTPs) GetPending(_ context.Context, userID uuid.UUID, purpose domain.OTPPurpose) (domain.OTPCode, error) {
	for i := len(m.items) - 1; i >= 0; i-- {
		if m.items[i].UserID == userID && m.items[i].Purpose == purpose && m.items[i].ConsumedAt == nil {
			return m.items[i], nil
		}
	}
	return domain.OTPCode{}, nf()
}

type memRefresh struct{ n int }

func (m *memRefresh) Create(context.Context, domain.RefreshToken) error { m.n++; return nil }
func (m *memRefresh) Revoke(context.Context, uuid.UUID) error           { return nil }
func (m *memRefresh) RevokeAllForUser(context.Context, uuid.UUID) error { return nil }
func (m *memRefresh) GetByHash(context.Context, string) (domain.RefreshToken, error) {
	return domain.RefreshToken{}, nf()
}

func TestRegisterThenMFA(t *testing.T) {
	initAuthCfg()
	users := newMemUsers()
	otps := &memOTPs{}
	sms := smsmock.New()
	reg := usecase.NewRegister(users, otps, sms)

	out, err := reg.Execute(context.Background(), input.Register{
		Email:    "a@b.com",
		Phone:    "+5554999000001",
		CPF:      "52998224725",
		Password: "senha-segura",
	})
	require.NoError(t, err)
	require.True(t, out.MFARequired)
	require.NotEmpty(t, out.MFAToken)

	msg, ok := sms.LastTo("+5554999000001")
	require.True(t, ok)
	code := otpDigits.FindString(msg.Body)
	require.Len(t, code, 6)

	mfa := usecase.NewMFAVerify(users, otps, otps, nil, &memRefresh{})
	toks, err := mfa.Execute(context.Background(), input.MFAVerify{MFAToken: out.MFAToken, Code: code})
	require.NoError(t, err)
	require.NotEmpty(t, toks.AccessToken)

	claims, err := auth.ParseAccess(toks.AccessToken)
	require.NoError(t, err)
	require.Equal(t, coredomain.RoleProducer, claims.RoleValue())
}

func TestLoginWrongPassword(t *testing.T) {
	initAuthCfg()
	users := newMemUsers()
	otps := &memOTPs{}
	sms := smsmock.New()
	_, err := usecase.NewRegister(users, otps, sms).Execute(context.Background(), input.Register{
		Email: "a@b.com", Phone: "+5554999000001", CPF: "52998224725", Password: "senha-segura",
	})
	require.NoError(t, err)

	login := usecase.NewLogin(users, otps, sms, nil, &memRefresh{})
	_, err = login.Execute(context.Background(), input.Login{Email: "a@b.com", Password: "erradaaaaa"})
	require.Error(t, err)
}

func lastSix(s string) string {
	return otpDigits.FindString(s)
}

var otpDigits = regexp.MustCompile(`\b\d{6}\b`)

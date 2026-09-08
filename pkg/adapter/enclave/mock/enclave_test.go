package mock_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/AgroBench/backend/internal/core/crypto"
	"github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/internal/core/payload"
	"github.com/AgroBench/backend/internal/core/validate"
	"github.com/AgroBench/backend/pkg/adapter/enclave/mock"
	"github.com/AgroBench/backend/pkg/port"
)

const nonce = "0123456789abcdef0123456789abcdef"

func newEnclave(t *testing.T) *mock.Enclave {
	t.Helper()
	e, err := mock.New(mock.Config{})
	require.NoError(t, err)
	return e
}

func sealed(t *testing.T, e *mock.Enclave, cycle uuid.UUID, level domain.ContributionLevel, data any) (cipher []byte, hash string) {
	t.Helper()
	plain, err := payload.Build(level, cycle, nonce, data)
	require.NoError(t, err)
	cipher, err = crypto.Seal(plain, e.BoxPublicKey())
	require.NoError(t, err)
	return cipher, payload.Hash(plain)
}

func TestValidateAcceptsAndSigns(t *testing.T) {
	e := newEnclave(t)
	cycle, contrib := uuid.New(), uuid.New()
	cipher, hash := sealed(t, e, cycle, domain.LevelBasic, payload.Basic{CultureCode: "soybean", AreaHa: 50, TotalCostBRL: 250_000})

	v, err := e.Validate(context.Background(), port.ValidateRequest{
		ContributionID: contrib, CycleID: cycle, Level: domain.LevelBasic, CommitHash: hash, Ciphertext: cipher,
		References: []port.ReferenceRange{{Metric: payload.MetricTotalCostHa, Min: 3000, Max: 8000}},
	})
	require.NoError(t, err)
	require.True(t, v.Accepted, "checks: %+v", v.Checks)
	require.Len(t, v.Metrics, 2)

	id, _ := e.Identity(context.Background())
	require.NoError(t, validate.Verify(v, id.SigningPublicKey))

	v.Accepted = false // adulteração → assinatura inválida
	require.Error(t, validate.Verify(v, id.SigningPublicKey))
}

func TestValidateRejectsOutOfRange(t *testing.T) {
	e := newEnclave(t)
	cycle := uuid.New()
	cipher, hash := sealed(t, e, cycle, domain.LevelBasic, payload.Basic{CultureCode: "soybean", AreaHa: 50, TotalCostBRL: 2_500_000})

	v, err := e.Validate(context.Background(), port.ValidateRequest{
		ContributionID: uuid.New(), CycleID: cycle, Level: domain.LevelBasic, CommitHash: hash, Ciphertext: cipher,
		References: []port.ReferenceRange{{Metric: payload.MetricTotalCostHa, Min: 3000, Max: 8000}},
	})
	require.NoError(t, err)
	require.False(t, v.Accepted)
	require.Empty(t, v.Metrics, "rejeitado não vaza métrica")
	require.Equal(t, "reference_range:total_cost_ha", v.Checks[len(v.Checks)-1].Name)
}

func TestValidateRejectsHashMismatchAndWrongKey(t *testing.T) {
	e := newEnclave(t)
	cycle := uuid.New()
	cipher, _ := sealed(t, e, cycle, domain.LevelBasic, payload.Basic{CultureCode: "soybean", AreaHa: 50, TotalCostBRL: 250_000})

	v, err := e.Validate(context.Background(), port.ValidateRequest{
		ContributionID: uuid.New(), CycleID: cycle, Level: domain.LevelBasic, CommitHash: "deadbeef", Ciphertext: cipher,
	})
	require.NoError(t, err)
	require.False(t, v.Accepted)
	require.Equal(t, validate.CheckHashMatchesCommit, v.Checks[0].Name)

	other := newEnclave(t) // cifrado pra outro enclave
	cipher2, hash2 := sealed(t, other, cycle, domain.LevelBasic, payload.Basic{CultureCode: "soybean", AreaHa: 50, TotalCostBRL: 250_000})
	v, err = e.Validate(context.Background(), port.ValidateRequest{
		ContributionID: uuid.New(), CycleID: cycle, Level: domain.LevelBasic, CommitHash: hash2, Ciphertext: cipher2,
	})
	require.NoError(t, err)
	require.False(t, v.Accepted)
	require.Equal(t, "decrypt", v.Checks[0].Name)
}

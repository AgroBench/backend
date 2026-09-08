package solana

import (
	"context"
	"errors"
	"testing"

	ag "github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"

	"github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/pkg/port"
)

func testConfig(t *testing.T) Config {
	t.Helper()
	pk, err := ag.NewRandomPrivateKey()
	require.NoError(t, err)
	return Config{
		RPCURL:             "https://api.devnet.solana.com",
		TreasuryPrivateKey: pk.String(),
		USDCMint:           "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU",
	}
}

func TestNewRequiresFields(t *testing.T) {
	_, err := New(Config{})
	require.Error(t, err)

	cfg := testConfig(t)
	cfg.USDCMint = "not-a-pubkey"
	_, err = New(cfg)
	require.Error(t, err)
}

func TestNewTreasuryAndPool(t *testing.T) {
	cfg := testConfig(t)
	c, err := New(cfg)
	require.NoError(t, err)
	require.Equal(t, c.Treasury(), c.Pool(), "pool vazio reusa a treasury")

	pk, err := ag.NewRandomPrivateKey()
	require.NoError(t, err)
	cfg.PoolPubkey = pk.PublicKey().String()
	c, err = New(cfg)
	require.NoError(t, err)
	require.Equal(t, port.Account(pk.PublicKey().String()), c.Pool())
	require.NotEqual(t, c.Treasury(), c.Pool())
}

func TestLockAndReleaseStakeNotImplemented(t *testing.T) {
	c, err := New(testConfig(t))
	require.NoError(t, err)

	_, err = c.LockStake(context.Background(), port.StakeRequest{})
	require.ErrorIs(t, err, port.ErrNotImplemented)

	_, err = c.ReleaseStake(context.Background(), port.StakeRequest{})
	require.ErrorIs(t, err, port.ErrNotImplemented)
}

func TestTransferUSDCRejectsNonTreasuryAndZero(t *testing.T) {
	c, err := New(testConfig(t))
	require.NoError(t, err)

	_, err = c.TransferUSDC(context.Background(), port.TransferRequest{
		From: "SomeOtherWallet111111111111111111111111111", Amount: domain.USDC(1),
	})
	require.ErrorIs(t, err, port.ErrNotImplemented)

	_, err = c.TransferUSDC(context.Background(), port.TransferRequest{
		From: c.Treasury(), Amount: 0,
	})
	require.Error(t, err)
	require.False(t, errors.Is(err, port.ErrNotImplemented))
}

func TestBalanceRejectsInvalidAccount(t *testing.T) {
	c, err := New(testConfig(t))
	require.NoError(t, err)
	_, err = c.Balance(context.Background(), "not-base58")
	require.Error(t, err)
}

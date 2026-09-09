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
	require.Equal(t, c.Treasury(), c.Pool(), "sem program_id, pool vazio reusa a treasury")
	require.Empty(t, c.ProgramID())

	pk, err := ag.NewRandomPrivateKey()
	require.NoError(t, err)
	cfg.PoolPubkey = pk.PublicKey().String()
	c, err = New(cfg)
	require.NoError(t, err)
	require.Equal(t, port.Account(pk.PublicKey().String()), c.Pool())
	require.NotEqual(t, c.Treasury(), c.Pool())
}

func TestNewDerivesPoolPDAFromProgram(t *testing.T) {
	cfg := testConfig(t)
	prog, err := ag.NewRandomPrivateKey()
	require.NoError(t, err)
	cfg.ProgramID = prog.PublicKey().String()
	cfg.PoolPubkey = "" // não deixar pool=treasury quando dá para derivar
	c, err := New(cfg)
	require.NoError(t, err)
	require.Equal(t, prog.PublicKey().String(), c.ProgramID())
	require.NotEqual(t, c.Treasury(), c.Pool())

	want, _, err := poolPDA(prog.PublicKey())
	require.NoError(t, err)
	require.Equal(t, port.Account(want.String()), c.Pool())
}

func TestLockStakeRequiresCoSignWhenProgramSet(t *testing.T) {
	cfg := testConfig(t)
	prog, err := ag.NewRandomPrivateKey()
	require.NoError(t, err)
	cfg.ProgramID = prog.PublicKey().String()
	c, err := New(cfg)
	require.NoError(t, err)

	_, err = c.LockStake(context.Background(), port.StakeRequest{Wallet: "Prod", Amount: domain.USDC(10)})
	require.ErrorIs(t, err, port.ErrNeedsCoSign)
	_, err = c.ReleaseStake(context.Background(), port.StakeRequest{Wallet: "Prod", Amount: domain.USDC(10)})
	require.ErrorIs(t, err, port.ErrNeedsCoSign)
}

func TestStakeMemoFormat(t *testing.T) {
	req := port.StakeRequest{
		Wallet: "ProdWallet1111111111111111111111111111111",
		Amount: domain.USDC(10),
	}
	require.Contains(t, stakeMemo("lock", req), "agrobench:v1:stake:lock:")
	require.Contains(t, stakeMemo("release", req), "agrobench:v1:stake:release:")
	require.Contains(t, stakeMemo("lock", req), "10000000") // 10 USDC em micro
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

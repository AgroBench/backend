package mock_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/internal/testutil"
	"github.com/AgroBench/backend/pkg/adapter/chain/mock"
	"github.com/AgroBench/backend/pkg/port"
)

func TestChainMockIntegration(t *testing.T) {
	db := testutil.Postgres(t)
	ctx := context.Background()
	c := mock.New(db, mock.Config{})
	wallet := port.Account("ProducerWallet1111111111111111111111111111")

	require.NoError(t, c.Mint(ctx, c.Treasury(), domain.USDC(1000)))

	// commit é idempotente por conteúdo
	req := port.CommitRequest{Wallet: wallet, ContributionID: uuid.New(), CycleID: uuid.New(), Hash: "abc"}
	ref1, err := c.Commit(ctx, req)
	require.NoError(t, err)
	ref2, err := c.Commit(ctx, req)
	require.NoError(t, err)
	require.Equal(t, ref1, ref2)

	// recompensa treasury → produtor
	_, err = c.TransferUSDC(ctx, port.TransferRequest{From: c.Treasury(), To: wallet, Amount: domain.USDC(5), Memo: "reward:test"})
	require.NoError(t, err)
	bal, err := c.Balance(ctx, wallet)
	require.NoError(t, err)
	require.Equal(t, domain.USDC(5), bal)

	// stake: sem saldo suficiente falha; com saldo trava e devolve
	stake := port.StakeRequest{Wallet: wallet, ContributionID: req.ContributionID, Amount: domain.USDC(10)}
	_, err = c.LockStake(ctx, stake)
	require.ErrorIs(t, err, mock.ErrInsufficientFunds)

	require.NoError(t, c.Mint(ctx, wallet, domain.USDC(10)))
	_, err = c.LockStake(ctx, stake)
	require.NoError(t, err)
	bal, _ = c.Balance(ctx, wallet)
	require.Equal(t, domain.USDC(5), bal)

	_, err = c.ReleaseStake(ctx, stake)
	require.NoError(t, err)
	bal, _ = c.Balance(ctx, wallet)
	require.Equal(t, domain.USDC(15), bal)

	_, err = c.ReleaseStake(ctx, stake)
	require.Error(t, err, "não libera duas vezes")

	var events int
	require.NoError(t, db.Get(&events, `SELECT count(*) FROM mock_chain_events`))
	require.Equal(t, 4, events) // commit, transfer, stake_lock, stake_release
}

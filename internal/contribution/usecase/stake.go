package usecase

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/viper"

	"github.com/AgroBench/backend/internal/apperrors"
	"github.com/AgroBench/backend/internal/contribution/contract"
	cdomain "github.com/AgroBench/backend/internal/contribution/domain"
	"github.com/AgroBench/backend/internal/contribution/types/input"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	walletcontract "github.com/AgroBench/backend/internal/wallet/contract"
	"github.com/AgroBench/backend/pkg/port"
)

type UnsignedTx struct {
	Tx string `json:"tx"`
}

type SubmittedTx struct {
	Signature string `json:"signature"`
}

type LockTx struct {
	wallets walletcontract.WalletRepo
	chain   port.ChainClient
}

func NewLockTx(wallets walletcontract.WalletRepo, chain port.ChainClient) *LockTx {
	return &LockTx{wallets: wallets, chain: chain}
}

func (u *LockTx) Execute(ctx context.Context, userID uuid.UUID, in input.LockStakeTx) (UnsignedTx, error) {
	const op = "contribution.LockTx"
	wallet, err := u.wallets.GetByUserID(ctx, userID)
	if err != nil {
		return UnsignedTx{}, err
	}
	amount := domainMicro(in.Amount)
	if err := requireStakeBalance(ctx, op, u.chain, wallet.Pubkey, amount); err != nil {
		return UnsignedTx{}, err
	}
	tx, err := u.chain.BuildLockStakeTx(ctx, port.StakeRequest{
		Wallet: port.Account(wallet.Pubkey), Amount: amount,
	})
	if err != nil {
		return UnsignedTx{}, apperrors.External(op, 0, err).WithDetail("falha ao montar lock de stake")
	}
	return UnsignedTx{Tx: tx}, nil
}

type LockSubmit struct {
	chain port.ChainClient
}

func NewLockSubmit(chain port.ChainClient) *LockSubmit { return &LockSubmit{chain: chain} }

func (u *LockSubmit) Execute(ctx context.Context, in input.SubmitSignedTx) (SubmittedTx, error) {
	const op = "contribution.LockSubmit"
	ref, err := u.chain.SubmitSignedTx(ctx, in.Tx)
	if err != nil {
		if errors.Is(err, port.ErrNeedsCoSign) {
			return SubmittedTx{}, apperrors.Validation(op, err).WithDetail("tx sem assinatura do produtor")
		}
		return SubmittedTx{}, apperrors.External(op, 0, err).WithDetail(stakeChainDetail(err, "falha ao enviar lock de stake"))
	}
	return SubmittedTx{Signature: string(ref)}, nil
}

type ReleaseTx struct {
	repo    contract.Repo
	wallets walletcontract.WalletRepo
	chain   port.ChainClient
}

func NewReleaseTx(repo contract.Repo, wallets walletcontract.WalletRepo, chain port.ChainClient) *ReleaseTx {
	return &ReleaseTx{repo: repo, wallets: wallets, chain: chain}
}

func (u *ReleaseTx) Execute(ctx context.Context, userID, contributionID uuid.UUID) (UnsignedTx, error) {
	const op = "contribution.ReleaseTx"
	st, wallet, err := u.lockedOwnStake(ctx, op, userID, contributionID)
	if err != nil {
		return UnsignedTx{}, err
	}
	tx, err := u.chain.BuildReleaseStakeTx(ctx, port.StakeRequest{
		Wallet: port.Account(wallet), ContributionID: st.ContributionID, Amount: coredomain.USDC(st.AmountUSDC),
	})
	if err != nil {
		return UnsignedTx{}, apperrors.External(op, 0, err).WithDetail("falha ao montar release de stake")
	}
	return UnsignedTx{Tx: tx}, nil
}

type ReleaseSubmit struct {
	repo    contract.Repo
	wallets walletcontract.WalletRepo
	chain   port.ChainClient
}

func NewReleaseSubmit(repo contract.Repo, wallets walletcontract.WalletRepo, chain port.ChainClient) *ReleaseSubmit {
	return &ReleaseSubmit{repo: repo, wallets: wallets, chain: chain}
}

func (u *ReleaseSubmit) Execute(ctx context.Context, userID, contributionID uuid.UUID, in input.SubmitSignedTx) (SubmittedTx, error) {
	const op = "contribution.ReleaseSubmit"
	st, _, err := u.lockedOwnStake(ctx, op, userID, contributionID)
	if err != nil {
		return SubmittedTx{}, err
	}
	ref, err := u.chain.SubmitSignedTx(ctx, in.Tx)
	if err != nil {
		if errors.Is(err, port.ErrNeedsCoSign) {
			return SubmittedTx{}, apperrors.Validation(op, err).WithDetail("tx sem assinatura do produtor")
		}
		return SubmittedTx{}, apperrors.External(op, 0, err).WithDetail("falha ao enviar release de stake")
	}
	if err := u.repo.ReleaseStake(ctx, st.ID, string(ref)); err != nil {
		return SubmittedTx{}, err
	}
	return SubmittedTx{Signature: string(ref)}, nil
}

func (u *ReleaseTx) lockedOwnStake(ctx context.Context, op string, userID, contributionID uuid.UUID) (cdomain.Stake, string, error) {
	return loadLockedOwnStake(ctx, op, u.repo, u.wallets, userID, contributionID)
}

func (u *ReleaseSubmit) lockedOwnStake(ctx context.Context, op string, userID, contributionID uuid.UUID) (cdomain.Stake, string, error) {
	return loadLockedOwnStake(ctx, op, u.repo, u.wallets, userID, contributionID)
}

func loadLockedOwnStake(ctx context.Context, op string, repo contract.Repo, wallets walletcontract.WalletRepo, userID, contributionID uuid.UUID) (cdomain.Stake, string, error) {
	c, err := repo.GetByID(ctx, contributionID)
	if err != nil {
		return cdomain.Stake{}, "", err
	}
	wallet, err := wallets.GetByUserID(ctx, userID)
	if err != nil {
		return cdomain.Stake{}, "", err
	}
	if c.WalletID != wallet.ID {
		return cdomain.Stake{}, "", apperrors.Forbidden(op, errors.New("wallet")).WithDetail("contribuição de outro usuário")
	}
	st, err := repo.GetStakeByContribution(ctx, contributionID)
	if err != nil {
		return cdomain.Stake{}, "", err
	}
	if st.Status != cdomain.StakeLocked {
		return cdomain.Stake{}, "", apperrors.Conflict(op, errors.New("stake")).WithDetail("stake não está travado")
	}
	return st, wallet.Pubkey, nil
}

func requireStakeBalance(ctx context.Context, op string, chain port.ChainClient, pubkey string, amount coredomain.MicroUSDC) error {
	if chain == nil || amount <= 0 {
		return nil
	}
	bal, err := chain.Balance(ctx, port.Account(pubkey))
	if err != nil {
		return apperrors.External(op, 0, err).WithDetail("falha ao consultar saldo USDC")
	}
	if bal < amount {
		return apperrors.Validation(op, errors.New("insufficient")).WithDetail(
			"saldo insuficiente para o lock de stake (carteira " + pubkey + " precisa de ≥10 USDC na Devnet)",
		)
	}
	return nil
}

func stakeChainDetail(err error, fallback string) string {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "accountnotinitialized") || strings.Contains(msg, "producer_ata") || strings.Contains(msg, "insufficient") {
		return "saldo insuficiente para o lock de stake"
	}
	return fallback
}

func domainMicro(amount int64) coredomain.MicroUSDC {
	if amount > 0 {
		return coredomain.MicroUSDC(amount)
	}
	v := viper.GetFloat64("stake.amount_usdc")
	if v <= 0 {
		v = 10
	}
	return coredomain.USDC(v)
}

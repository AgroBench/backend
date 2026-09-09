package usecase

import (
	"context"
	"encoding/base64"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/viper"

	"github.com/AgroBench/backend/internal/apperrors"
	carcontract "github.com/AgroBench/backend/internal/car/contract"
	"github.com/AgroBench/backend/internal/car/domain"
	"github.com/AgroBench/backend/internal/contribution/contract"
	cdomain "github.com/AgroBench/backend/internal/contribution/domain"
	"github.com/AgroBench/backend/internal/contribution/types/input"
	"github.com/AgroBench/backend/internal/contribution/types/output"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	cyclecontract "github.com/AgroBench/backend/internal/cycle/contract"
	walletcontract "github.com/AgroBench/backend/internal/wallet/contract"
	"github.com/AgroBench/backend/pkg/adapter/queue"
	"github.com/AgroBench/backend/pkg/port"
)

type Commit struct {
	repo    contract.Repo
	cycles  cyclecontract.CycleRepo
	props   carcontract.PropertyRepo
	wallets walletcontract.WalletRepo
	chain   port.ChainClient
}

func NewCommit(repo contract.Repo, cycles cyclecontract.CycleRepo, props carcontract.PropertyRepo, wallets walletcontract.WalletRepo, chain port.ChainClient) *Commit {
	return &Commit{repo: repo, cycles: cycles, props: props, wallets: wallets, chain: chain}
}

func (u *Commit) Execute(ctx context.Context, userID uuid.UUID, in input.Commit) (output.Contribution, error) {
	const op = "contribution.Commit"
	level, err := coredomain.ParseLevel(in.Level)
	if err != nil {
		return output.Contribution{}, apperrors.Validation(op, err)
	}
	cycle, err := u.cycles.GetByID(ctx, in.CycleID)
	if err != nil {
		return output.Contribution{}, err
	}
	if !cycle.IsOpen(time.Now()) {
		return output.Contribution{}, apperrors.Conflict(op, errors.New("closed")).WithDetail("ciclo não está aberto")
	}
	prop, err := u.props.GetByID(ctx, in.PropertyID)
	if err != nil {
		return output.Contribution{}, err
	}
	if prop.UserID != userID {
		return output.Contribution{}, apperrors.Forbidden(op, errors.New("property")).WithDetail("propriedade de outro usuário")
	}
	if prop.CARStatus != domain.CARApproved {
		return output.Contribution{}, apperrors.Forbidden(op, errors.New("car")).WithDetail("CAR não aprovado")
	}
	wallet, err := u.wallets.GetByUserID(ctx, userID)
	if err != nil {
		return output.Contribution{}, err
	}
	attempt := 1
	if _, err := u.repo.ActiveInCycle(ctx, in.CycleID, wallet.ID); err == nil {
		return output.Contribution{}, apperrors.Conflict(op, errors.New("active")).WithDetail("já existe contribuição ativa neste ciclo")
	} else if !apperrors.Is(err, apperrors.ErrNotFound) {
		return output.Contribution{}, err
	}
	rejected, err := u.repo.HasRejected(ctx, in.CycleID, wallet.ID)
	if err != nil {
		return output.Contribution{}, err
	}
	if rejected {
		attempt = 2
	}
	c := cdomain.Contribution{
		ID: coredomain.NewID(), CycleID: in.CycleID, WalletID: wallet.ID, PropertyID: in.PropertyID,
		Level: level, Attempt: attempt, CommitHash: in.Hash, CommitAt: time.Now(), Status: cdomain.StatusCommitted,
	}

	var stakeLocked bool
	var stakeReq port.StakeRequest
	var lockTx port.TxRef
	amount := viper.GetFloat64("stake.amount_usdc")
	if amount <= 0 {
		amount = 10
	}
	if attempt == 1 {
		stakeReq = port.StakeRequest{
			Wallet: port.Account(wallet.Pubkey), ContributionID: c.ID, Amount: coredomain.USDC(amount),
		}
		switch {
		case in.SignedTx != "":
			lockTx, err = u.chain.SubmitSignedTx(ctx, in.SignedTx)
			if err != nil {
				return output.Contribution{}, mapChainErr(op, err, "falha ao enviar lock de stake")
			}
		case in.StakeTx != "":
			lockTx = port.TxRef(in.StakeTx)
		}
	}

	tx, err := u.chain.Commit(ctx, port.CommitRequest{
		Wallet: port.Account(wallet.Pubkey), ContributionID: c.ID, CycleID: in.CycleID, Hash: in.Hash,
	})
	if err != nil {
		return output.Contribution{}, apperrors.External(op, 0, err).WithDetail("falha no commit on-chain")
	}
	c.CommitTx = string(tx)

	err = u.repo.WithTx(ctx, func(txRepo contract.Repo) error {
		if err := txRepo.Create(ctx, c); err != nil {
			return err
		}
		if attempt != 1 {
			return nil
		}
		if lockTx == "" {
			var err error
			lockTx, err = u.chain.LockStake(ctx, stakeReq)
			if err != nil {
				return mapChainErr(op, err, "falha ao travar stake (saldo insuficiente?)")
			}
			stakeLocked = true
		}
		return txRepo.CreateStake(ctx, cdomain.Stake{
			ID: coredomain.NewID(), ContributionID: c.ID, AmountUSDC: amount, LockTx: string(lockTx), Status: cdomain.StakeLocked,
		})
	})
	if err != nil {
		if stakeLocked {
			if _, relErr := u.chain.ReleaseStake(ctx, stakeReq); relErr != nil {
				err = errors.Join(err, relErr)
			}
		}
		return output.Contribution{}, err
	}
	return output.New(c), nil
}

func mapChainErr(op string, err error, detail string) error {
	if errors.Is(err, port.ErrNeedsCoSign) {
		return apperrors.Validation(op, err).WithDetail("assine o lock de stake antes do commit (POST /chain/stake/lock-tx)")
	}
	return apperrors.External(op, 0, err).WithDetail(detail)
}

type Reveal struct {
	repo  contract.Repo
	queue *queue.Queue
}

func NewReveal(repo contract.Repo, q *queue.Queue) *Reveal { return &Reveal{repo: repo, queue: q} }

func (u *Reveal) Execute(ctx context.Context, userID, id uuid.UUID, in input.Reveal) (output.Contribution, error) {
	const op = "contribution.Reveal"
	c, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return output.Contribution{}, err
	}
	if c.Status != cdomain.StatusCommitted {
		return output.Contribution{}, apperrors.Conflict(op, errors.New("status")).WithDetail("contribuição não está em committed")
	}
	blob, err := base64.StdEncoding.DecodeString(in.Ciphertext)
	if err != nil {
		return output.Contribution{}, apperrors.Validation(op, err).WithDetail("ciphertext deve ser base64")
	}
	if err := u.repo.UpdateReveal(ctx, c.ID, blob); err != nil {
		return output.Contribution{}, err
	}
	if u.queue != nil {
		if err := u.queue.EnqueueValidate(ctx, c.ID); err != nil {
			return output.Contribution{}, apperrors.Internal(op, err)
		}
	}
	c.Status = cdomain.StatusRevealed
	return output.New(c), nil
}

type List struct{ repo contract.Repo }

func NewList(repo contract.Repo) *List { return &List{repo: repo} }

func (u *List) Execute(ctx context.Context, walletID uuid.UUID) ([]output.Contribution, error) {
	list, err := u.repo.ListByWallet(ctx, walletID)
	if err != nil {
		return nil, err
	}
	out := make([]output.Contribution, len(list))
	for i, c := range list {
		out[i] = output.New(c)
	}
	return out, nil
}

type Get struct{ repo contract.Repo }

func NewGet(repo contract.Repo) *Get { return &Get{repo: repo} }

func (u *Get) Execute(ctx context.Context, id uuid.UUID) (output.Contribution, error) {
	c, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return output.Contribution{}, err
	}
	return output.New(c), nil
}

type EnclaveKey struct{ enc port.Enclave }

func NewEnclaveKey(enc port.Enclave) *EnclaveKey { return &EnclaveKey{enc: enc} }

func (u *EnclaveKey) Execute(ctx context.Context) (output.EnclaveKey, error) {
	id, err := u.enc.Identity(ctx)
	if err != nil {
		return output.EnclaveKey{}, apperrors.External("enclave.Identity", 0, err)
	}
	return output.EnclaveKey{
		BoxPublicKey: id.BoxPublicKey, SigningPublicKey: id.SigningPublicKey,
		Provider: id.Provider, Attestation: id.Attestation,
	}, nil
}

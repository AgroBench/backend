// Package solana é a implementação REAL de port.ChainClient (devnet no pitch).
//
// Com CHAIN_SOLANA_PROGRAM_ID:
//   - lock_stake / release_stake no programa agrobench (USDC na vault PDA).
//     O produtor assina no device; a treasury é fee payer e co-assina.
//   - credit_pool e distribute on-chain (pool = PDA ["pool"]).
//   - Commit / atestação / CAR continuam Memo (commit-reveal).
//
// Sem program id (fallback da demo se o programa ainda não fez deploy):
//   - Memo + SPL da treasury, como antes.
package solana

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	ag "github.com/gagliardetto/solana-go"
	ata "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/programs/memo"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"

	"github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/pkg/port"
)

var _ port.ChainClient = (*Chain)(nil)

const memoPrefix = "agrobench:v1"

type Config struct {
	RPCURL             string        // ex.: https://api.devnet.solana.com
	TreasuryPrivateKey string        // base58 (SOLANA_TREASURY_PRIVATE_KEY no .env)
	USDCMint           string        // devnet: 4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU
	PoolPubkey         string        // ignorado se ProgramID está setado (usa PDA)
	ProgramID          string        // agrobench; vazio = fallback Memo/SPL
	ConfirmTimeout     time.Duration // default 60s
}

type Chain struct {
	rpc      *rpc.Client
	treasury ag.PrivateKey
	mint     ag.PublicKey
	pool     ag.PublicKey
	program  ag.PublicKey
	timeout  time.Duration
}

func New(cfg Config) (*Chain, error) {
	if cfg.RPCURL == "" || cfg.TreasuryPrivateKey == "" || cfg.USDCMint == "" {
		return nil, errors.New("chain solana: rpc_url, treasury_private_key e usdc_mint são obrigatórios")
	}
	treasury, err := ag.PrivateKeyFromBase58(strings.TrimSpace(cfg.TreasuryPrivateKey))
	if err != nil {
		return nil, fmt.Errorf("chain solana: treasury_private_key: %w", err)
	}
	mint, err := ag.PublicKeyFromBase58(cfg.USDCMint)
	if err != nil {
		return nil, fmt.Errorf("chain solana: usdc_mint: %w", err)
	}

	var program ag.PublicKey
	pool := treasury.PublicKey()
	if strings.TrimSpace(cfg.ProgramID) != "" {
		program, err = ag.PublicKeyFromBase58(strings.TrimSpace(cfg.ProgramID))
		if err != nil {
			return nil, fmt.Errorf("chain solana: program_id: %w", err)
		}
		derived, _, err := poolPDA(program)
		if err != nil {
			return nil, fmt.Errorf("chain solana: pool PDA: %w", err)
		}
		pool = derived
	} else if cfg.PoolPubkey != "" {
		if pool, err = ag.PublicKeyFromBase58(cfg.PoolPubkey); err != nil {
			return nil, fmt.Errorf("chain solana: pool_pubkey: %w", err)
		}
	}
	if cfg.ConfirmTimeout == 0 {
		cfg.ConfirmTimeout = 60 * time.Second
	}
	return &Chain{
		rpc: rpc.New(cfg.RPCURL), treasury: treasury, mint: mint,
		pool: pool, program: program, timeout: cfg.ConfirmTimeout,
	}, nil
}

func (c *Chain) Treasury() port.Account { return port.Account(c.treasury.PublicKey().String()) }
func (c *Chain) Pool() port.Account     { return port.Account(c.pool.String()) }

func (c *Chain) Commit(ctx context.Context, req port.CommitRequest) (port.TxRef, error) {
	return c.sendMemo(ctx, fmt.Sprintf("%s:commit:%s:%s:%s", memoPrefix, req.Wallet, req.CycleID, req.Hash))
}

func (c *Chain) Attest(ctx context.Context, req port.AttestRequest) (port.TxRef, error) {
	return c.sendMemo(ctx, fmt.Sprintf("%s:attest:%s:%s:%s:%t", memoPrefix, req.Wallet, req.Hash, req.Level, req.Accepted))
}

func (c *Chain) RecordCARVerification(ctx context.Context, wallet port.Account, approved bool) (port.TxRef, error) {
	return c.sendMemo(ctx, fmt.Sprintf("%s:car:%s:%t", memoPrefix, wallet, approved))
}

// TransferUSDC só consegue assinar saídas da treasury. Com programa, crédito no pool
// e payouts passam por credit_pool / distribute.
func (c *Chain) TransferUSDC(ctx context.Context, req port.TransferRequest) (port.TxRef, error) {
	if req.Amount <= 0 {
		return "", errors.New("chain solana: valor deve ser positivo")
	}
	if c.hasProgram() && req.To == c.Pool() && req.From == c.Treasury() {
		return c.CreditPool(ctx, req.Amount, req.Memo)
	}
	if c.hasProgram() && req.From == c.Pool() {
		return c.DistributePool(ctx, []port.PoolPayout{{Wallet: req.To, Amount: req.Amount}})
	}
	if req.From != c.Treasury() {
		return "", fmt.Errorf("%w: TransferUSDC a partir de %s (só a treasury pode assinar)", port.ErrNotImplemented, req.From)
	}
	// Pool == treasury (fallback sem programa): não há conta de destino distinta.
	if req.To == c.Treasury() {
		return c.sendMemo(ctx, fmt.Sprintf("%s:%s", memoPrefix, req.Memo))
	}
	to, err := ag.PublicKeyFromBase58(string(req.To))
	if err != nil {
		return "", fmt.Errorf("chain solana: destino inválido: %w", err)
	}
	owner := c.treasury.PublicKey()
	srcATA, _, err := ag.FindAssociatedTokenAddress(owner, c.mint)
	if err != nil {
		return "", err
	}
	dstATA, _, err := ag.FindAssociatedTokenAddress(to, c.mint)
	if err != nil {
		return "", err
	}

	var instrs []ag.Instruction
	exists, err := c.accountExists(ctx, dstATA)
	if err != nil {
		return "", err
	}
	if !exists {
		instrs = append(instrs, ata.NewCreateInstruction(owner, to, c.mint).Build())
	}
	instrs = append(instrs,
		token.NewTransferInstruction(uint64(req.Amount), srcATA, dstATA, owner, nil).Build(),
		memo.NewMemoInstruction([]byte(memoPrefix+":"+req.Memo), owner).Build(),
	)
	return c.sendAndConfirm(ctx, instrs)
}

func (c *Chain) LockStake(ctx context.Context, req port.StakeRequest) (port.TxRef, error) {
	if c.hasProgram() {
		return "", fmt.Errorf("%w: use POST /chain/stake/lock-tx", port.ErrNeedsCoSign)
	}
	return c.sendMemo(ctx, stakeMemo("lock", req))
}

func (c *Chain) ReleaseStake(ctx context.Context, req port.StakeRequest) (port.TxRef, error) {
	if c.hasProgram() {
		return "", fmt.Errorf("%w: use POST /contributions/{id}/release-stake/tx", port.ErrNeedsCoSign)
	}
	return c.sendMemo(ctx, stakeMemo("release", req))
}

func stakeMemo(kind string, req port.StakeRequest) string {
	return fmt.Sprintf("%s:stake:%s:%s:%s:%d", memoPrefix, kind, req.Wallet, req.ContributionID, req.Amount)
}

func (c *Chain) Balance(ctx context.Context, account port.Account) (domain.MicroUSDC, error) {
	owner, err := ag.PublicKeyFromBase58(string(account))
	if err != nil {
		return 0, fmt.Errorf("chain solana: conta inválida: %w", err)
	}
	acc, _, err := ag.FindAssociatedTokenAddress(owner, c.mint)
	if err != nil {
		return 0, err
	}
	res, err := c.rpc.GetTokenAccountBalance(ctx, acc, rpc.CommitmentConfirmed)
	if err != nil {
		if isMissingAccount(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("chain solana: saldo: %w", err)
	}
	if res == nil || res.Value == nil {
		return 0, nil
	}
	// USDC tem 6 decimais: Amount (menor unidade) == micro-USDC.
	raw, err := strconv.ParseInt(res.Value.Amount, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("chain solana: saldo inválido %q: %w", res.Value.Amount, err)
	}
	return domain.MicroUSDC(raw), nil
}

// --- internos ---

func (c *Chain) sendMemo(ctx context.Context, message string) (port.TxRef, error) {
	instr := memo.NewMemoInstruction([]byte(message), c.treasury.PublicKey()).Build()
	return c.sendAndConfirm(ctx, []ag.Instruction{instr})
}

func (c *Chain) sendAndConfirm(ctx context.Context, instrs []ag.Instruction) (port.TxRef, error) {
	bh, err := c.rpc.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return "", fmt.Errorf("chain solana: blockhash: %w", err)
	}
	tx, err := ag.NewTransaction(instrs, bh.Value.Blockhash, ag.TransactionPayer(c.treasury.PublicKey()))
	if err != nil {
		return "", fmt.Errorf("chain solana: montando tx: %w", err)
	}
	if _, err := tx.Sign(c.treasurySigner); err != nil {
		return "", fmt.Errorf("chain solana: assinando tx: %w", err)
	}

	sig, err := c.rpc.SendTransactionWithOpts(ctx, tx, rpc.TransactionOpts{PreflightCommitment: rpc.CommitmentConfirmed})
	if err != nil {
		return "", fmt.Errorf("chain solana: enviando tx: %w", err)
	}
	if err := c.waitConfirmed(ctx, sig); err != nil {
		return "", err
	}
	return port.TxRef(sig.String()), nil
}

func (c *Chain) waitConfirmed(ctx context.Context, sig ag.Signature) error {
	deadline := time.Now().Add(c.timeout)
	for time.Now().Before(deadline) {
		res, err := c.rpc.GetSignatureStatuses(ctx, false, sig)
		if err == nil && len(res.Value) > 0 && res.Value[0] != nil {
			st := res.Value[0]
			if st.Err != nil {
				return fmt.Errorf("chain solana: tx %s falhou on-chain: %v", sig, st.Err)
			}
			if st.ConfirmationStatus == rpc.ConfirmationStatusConfirmed || st.ConfirmationStatus == rpc.ConfirmationStatusFinalized {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1500 * time.Millisecond):
		}
	}
	return fmt.Errorf("chain solana: timeout aguardando confirmação de %s", sig)
}

func (c *Chain) accountExists(ctx context.Context, acc ag.PublicKey) (bool, error) {
	info, err := c.rpc.GetAccountInfo(ctx, acc)
	if errors.Is(err, rpc.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("chain solana: account info: %w", err)
	}
	return info != nil && info.Value != nil, nil
}

func isMissingAccount(err error) bool {
	if errors.Is(err, rpc.ErrNotFound) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "could not find account")
}

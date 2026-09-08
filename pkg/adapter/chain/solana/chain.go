// Package solana é a implementação REAL de port.ChainClient (NÃO ativa no pitch).
//
// Estratégia sem programa on-chain próprio (README §2, "Chain real"):
//   - Commit, atestação e resultado de CAR são transações com o Memo program, assinadas e
//     pagas pela treasury do protocolo (o produtor nunca precisa de SOL — README raiz §6.1).
//   - Recompensas e payouts são transferências USDC-SPL da ATA da treasury para a ATA da
//     wallet do produtor (criada pela treasury se não existir).
//   - LockStake/ReleaseStake exigem que o PRODUTOR assine (é o USDC dele). Como a chave
//     privada nunca chega ao backend (README raiz §6.1), o fluxo real precisa de uma transação
//     pré-assinada pelo app → fica ErrNotImplemented até esse fluxo existir. Evolução: um
//     programa Anchor de escrow.
//
// Status: semi-pronto. Compila e segue a API do solana-go v1.23, mas não foi executado
// contra a devnet dentro do escopo do MVP.
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
	PoolPubkey         string        // wallet do pool; no MVP pode ser a própria treasury
	ConfirmTimeout     time.Duration // default 60s
}

type Chain struct {
	rpc      *rpc.Client
	treasury ag.PrivateKey
	mint     ag.PublicKey
	pool     ag.PublicKey
	timeout  time.Duration
}

func New(cfg Config) (*Chain, error) {
	if cfg.RPCURL == "" || cfg.TreasuryPrivateKey == "" || cfg.USDCMint == "" {
		return nil, errors.New("chain solana: rpc_url, treasury_private_key e usdc_mint são obrigatórios")
	}
	treasury, err := ag.PrivateKeyFromBase58(cfg.TreasuryPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("chain solana: treasury_private_key: %w", err)
	}
	mint, err := ag.PublicKeyFromBase58(cfg.USDCMint)
	if err != nil {
		return nil, fmt.Errorf("chain solana: usdc_mint: %w", err)
	}
	pool := treasury.PublicKey()
	if cfg.PoolPubkey != "" {
		if pool, err = ag.PublicKeyFromBase58(cfg.PoolPubkey); err != nil {
			return nil, fmt.Errorf("chain solana: pool_pubkey: %w", err)
		}
	}
	if cfg.ConfirmTimeout == 0 {
		cfg.ConfirmTimeout = 60 * time.Second
	}
	return &Chain{rpc: rpc.New(cfg.RPCURL), treasury: treasury, mint: mint, pool: pool, timeout: cfg.ConfirmTimeout}, nil
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

// TransferUSDC só consegue assinar saídas da treasury (e do pool, quando pool == treasury).
func (c *Chain) TransferUSDC(ctx context.Context, req port.TransferRequest) (port.TxRef, error) {
	if req.Amount <= 0 {
		return "", errors.New("chain solana: valor deve ser positivo")
	}
	if req.From != c.Treasury() {
		return "", fmt.Errorf("%w: TransferUSDC a partir de %s (só a treasury pode assinar)", port.ErrNotImplemented, req.From)
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

func (c *Chain) LockStake(context.Context, port.StakeRequest) (port.TxRef, error) {
	return "", fmt.Errorf("%w: LockStake exige assinatura do produtor (ver comentário do pacote)", port.ErrNotImplemented)
}

func (c *Chain) ReleaseStake(context.Context, port.StakeRequest) (port.TxRef, error) {
	return "", fmt.Errorf("%w: ReleaseStake exige escrow on-chain (ver comentário do pacote)", port.ErrNotImplemented)
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
	if _, err := tx.Sign(func(key ag.PublicKey) *ag.PrivateKey {
		if key.Equals(c.treasury.PublicKey()) {
			return &c.treasury
		}
		return nil
	}); err != nil {
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

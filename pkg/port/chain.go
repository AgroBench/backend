package port

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/core/domain"
)

// Account é uma chave pública Solana em base58 (wallet do produtor, treasury ou pool).
type Account string

// TxRef identifica uma transação on-chain (signature em base58 na Solana; fake no mock).
type TxRef string

type CommitRequest struct {
	Wallet         Account
	ContributionID uuid.UUID
	CycleID        uuid.UUID
	Hash           string // sha256 hex do payload
}

type AttestRequest struct {
	Wallet         Account
	ContributionID uuid.UUID
	CycleID        uuid.UUID
	Hash           string
	Level          domain.ContributionLevel
	Accepted       bool
}

type TransferRequest struct {
	From   Account
	To     Account
	Amount domain.MicroUSDC
	Memo   string // ex.: "reward:<contribution_id>" ou "payout:<pool_period_id>"
}

type StakeRequest struct {
	Wallet         Account
	ContributionID uuid.UUID
	Amount         domain.MicroUSDC
}

type PoolPayout struct {
	Wallet Account
	Amount domain.MicroUSDC
}

// ErrNeedsCoSign indica que a ix exige a assinatura do produtor. O backend monta a
// tx (fee payer = treasury) e o front assina; depois SubmitSignedTx co-assina e envia.
var ErrNeedsCoSign = errors.New("port: operação exige co-assinatura do produtor")

// ChainClient abstrai a blockchain Solana (README raiz §2, §5, §7).
//
// Implementações:
//   - pkg/adapter/chain/mock   → testes e seed. Grava cada operação em mock_chain_events e
//     mantém saldos em mock_chain_balances, tudo no Postgres. TxRef é determinístico.
//   - pkg/adapter/chain/solana → ATIVA NA DEMO. Devnet via gagliardetto/solana-go.
//     Com CHAIN_SOLANA_PROGRAM_ID: lock/release/credit/distribute no programa agrobench.
//     Sem program id: fallback Memo + SPL da treasury (demo sobe sem deploy).
//
// Seleção: config `adapters.chain` (mock | solana).
type ChainClient interface {
	// Commit publica o hash do payload antes do reveal.
	Commit(ctx context.Context, req CommitRequest) (TxRef, error)
	// Attest registra que o hash foi validado (ou rejeitado) e compõe o agregado.
	Attest(ctx context.Context, req AttestRequest) (TxRef, error)
	// RecordCARVerification registra só o resultado (aprovado/rejeitado) da checagem de CAR — nunca o CAR.
	RecordCARVerification(ctx context.Context, wallet Account, approved bool) (TxRef, error)
	// TransferUSDC move USDC entre contas (treasury → wallet, instituição → pool, pool → wallet).
	TransferUSDC(ctx context.Context, req TransferRequest) (TxRef, error)
	// LockStake trava o depósito reembolsável do produtor; ReleaseStake devolve.
	// Com programa Anchor, estes métodos devolvem ErrNeedsCoSign — use Build*Tx + SubmitSignedTx.
	LockStake(ctx context.Context, req StakeRequest) (TxRef, error)
	ReleaseStake(ctx context.Context, req StakeRequest) (TxRef, error)
	// Balance devolve o saldo USDC de uma conta.
	Balance(ctx context.Context, account Account) (domain.MicroUSDC, error)
	// Treasury e Pool são as contas do protocolo. Pool é o PDA ["pool"] quando o programa está configurado.
	Treasury() Account
	Pool() Account

	// ProgramID é o id Anchor (base58) ou vazio no fallback Memo/SPL.
	ProgramID() string
	// BuildLockStakeTx / BuildReleaseStakeTx devolvem a tx serializada em base64
	// (fee payer = treasury, já parcial-assinada). O produtor assina no device.
	BuildLockStakeTx(ctx context.Context, req StakeRequest) (txBase64 string, err error)
	BuildReleaseStakeTx(ctx context.Context, req StakeRequest) (txBase64 string, err error)
	// SubmitSignedTx co-assina com a treasury se faltar e envia.
	SubmitSignedTx(ctx context.Context, txBase64 string) (TxRef, error)
	// CreditPool chama credit_pool on-chain (ou TransferUSDC no fallback).
	CreditPool(ctx context.Context, amount domain.MicroUSDC, memo string) (TxRef, error)
	// DistributePool chama distribute com ATAs + amounts (ou TransferUSDC no fallback).
	DistributePool(ctx context.Context, payouts []PoolPayout) (TxRef, error)
	// InitializeProgram chama initialize uma vez (no-op / erro se program id vazio).
	InitializeProgram(ctx context.Context) (TxRef, error)
}

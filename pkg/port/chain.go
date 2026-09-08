package port

import (
	"context"

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

// ChainClient abstrai a blockchain Solana (README raiz §2, §5, §7).
//
// Implementações:
//   - pkg/adapter/chain/mock   → ATIVA NO PITCH. Grava cada operação em mock_chain_events e
//     mantém saldos em mock_chain_balances, tudo no Postgres. TxRef é determinístico.
//   - pkg/adapter/chain/solana → REAL. Devnet via gagliardetto/solana-go. Commit e atestação
//     são transações com Memo program assinadas pela treasury do protocolo (que paga o gas);
//     recompensas e payouts são transferências USDC-SPL da treasury. LockStake/ReleaseStake
//     exigem assinatura do produtor e ficam ErrNotImplemented até existir o fluxo no app.
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
	LockStake(ctx context.Context, req StakeRequest) (TxRef, error)
	ReleaseStake(ctx context.Context, req StakeRequest) (TxRef, error)
	// Balance devolve o saldo USDC de uma conta.
	Balance(ctx context.Context, account Account) (domain.MicroUSDC, error)
	// Treasury e Pool são as contas do protocolo.
	Treasury() Account
	Pool() Account
}

// Package mock é a implementação de port.ChainClient ATIVA NO PITCH.
// Toda operação vira uma linha em mock_chain_events; saldos vivem em mock_chain_balances.
// O TxRef é determinístico (hash do conteúdo), então re-executar a mesma operação no mesmo
// contexto gera o mesmo id — parecido com a idempotência de uma tx real.
package mock

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/pkg/adapter/database"
	"github.com/AgroBench/backend/pkg/port"
)

const (
	kindCommit       = "commit"
	kindAttest       = "attest"
	kindCAR          = "car_verification"
	kindTransfer     = "transfer"
	kindStakeLock    = "stake_lock"
	kindStakeRelease = "stake_release"
)

var ErrInsufficientFunds = errors.New("chain mock: saldo insuficiente")

type Chain struct {
	db       *sqlx.DB
	treasury port.Account
	pool     port.Account
}

type Config struct {
	Treasury port.Account // qualquer string base58-like; default "MOCKTREASURY..."
	Pool     port.Account
}

func New(db *sqlx.DB, cfg Config) *Chain {
	if cfg.Treasury == "" {
		cfg.Treasury = "MockTreasury11111111111111111111111111111111"
	}
	if cfg.Pool == "" {
		cfg.Pool = "MockPool1111111111111111111111111111111111111"
	}
	return &Chain{db: db, treasury: cfg.Treasury, pool: cfg.Pool}
}

func (c *Chain) Treasury() port.Account { return c.treasury }
func (c *Chain) Pool() port.Account     { return c.pool }

func (c *Chain) Commit(ctx context.Context, req port.CommitRequest) (port.TxRef, error) {
	return c.record(ctx, c.db, kindCommit, req.Wallet, req)
}

func (c *Chain) Attest(ctx context.Context, req port.AttestRequest) (port.TxRef, error) {
	return c.record(ctx, c.db, kindAttest, req.Wallet, req)
}

func (c *Chain) RecordCARVerification(ctx context.Context, wallet port.Account, approved bool) (port.TxRef, error) {
	return c.record(ctx, c.db, kindCAR, wallet, map[string]any{"wallet": wallet, "approved": approved, "at": time.Now().UTC()})
}

func (c *Chain) TransferUSDC(ctx context.Context, req port.TransferRequest) (port.TxRef, error) {
	if req.Amount <= 0 {
		return "", fmt.Errorf("chain mock: valor deve ser positivo")
	}
	var ref port.TxRef
	err := database.WithTx(ctx, c.db, func(tx *sqlx.Tx) error {
		if err := debit(ctx, tx, req.From, req.Amount); err != nil {
			return err
		}
		if err := credit(ctx, tx, req.To, req.Amount); err != nil {
			return err
		}
		var err error
		ref, err = c.record(ctx, tx, kindTransfer, "", map[string]any{
			"from": req.From, "to": req.To, "micro_usdc": req.Amount, "memo": req.Memo, "at": time.Now().UTC(),
		})
		return err
	})
	return ref, err
}

func (c *Chain) LockStake(ctx context.Context, req port.StakeRequest) (port.TxRef, error) {
	var ref port.TxRef
	err := database.WithTx(ctx, c.db, func(tx *sqlx.Tx) error {
		if err := debit(ctx, tx, req.Wallet, req.Amount); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO mock_chain_stakes (contribution_id, wallet, micro_usdc) VALUES ($1, $2, $3)`,
			req.ContributionID, req.Wallet, req.Amount); err != nil {
			return fmt.Errorf("chain mock: lock stake: %w", err)
		}
		var err error
		ref, err = c.record(ctx, tx, kindStakeLock, req.Wallet, req)
		return err
	})
	return ref, err
}

func (c *Chain) ReleaseStake(ctx context.Context, req port.StakeRequest) (port.TxRef, error) {
	var ref port.TxRef
	err := database.WithTx(ctx, c.db, func(tx *sqlx.Tx) error {
		var amount domain.MicroUSDC
		err := tx.GetContext(ctx, &amount,
			`UPDATE mock_chain_stakes SET released = true
			  WHERE contribution_id = $1 AND wallet = $2 AND released = false
			  RETURNING micro_usdc`, req.ContributionID, req.Wallet)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("chain mock: stake não encontrado ou já liberado")
		}
		if err != nil {
			return fmt.Errorf("chain mock: release stake: %w", err)
		}
		if err := credit(ctx, tx, req.Wallet, amount); err != nil {
			return err
		}
		ref, err = c.record(ctx, tx, kindStakeRelease, req.Wallet, req)
		return err
	})
	return ref, err
}

func (c *Chain) Balance(ctx context.Context, account port.Account) (domain.MicroUSDC, error) {
	var bal domain.MicroUSDC
	err := c.db.GetContext(ctx, &bal, `SELECT micro_usdc FROM mock_chain_balances WHERE account = $1`, account)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return bal, err
}

// Mint cria USDC do nada numa conta. Só existe no mock: usado por seeds (treasury inicial,
// saldo de stake do produtor de demo, pagamento da instituição).
func (c *Chain) Mint(ctx context.Context, account port.Account, amount domain.MicroUSDC) error {
	return database.WithTx(ctx, c.db, func(tx *sqlx.Tx) error {
		return credit(ctx, tx, account, amount)
	})
}

// --- helpers ---

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (c *Chain) record(ctx context.Context, ex execer, kind string, wallet port.Account, payload any) (port.TxRef, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(append([]byte(kind+":"), raw...))
	ref := port.TxRef("mock_" + hex.EncodeToString(sum[:16]))

	var w *string
	if wallet != "" {
		s := string(wallet)
		w = &s
	}
	_, err = ex.ExecContext(ctx,
		`INSERT INTO mock_chain_events (id, tx_ref, kind, wallet, payload) VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (tx_ref) DO NOTHING`,
		domain.NewID(), ref, kind, w, raw)
	if err != nil {
		return "", fmt.Errorf("chain mock: gravando evento %s: %w", kind, err)
	}
	return ref, nil
}

func credit(ctx context.Context, tx *sqlx.Tx, account port.Account, amount domain.MicroUSDC) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO mock_chain_balances (account, micro_usdc) VALUES ($1, $2)
		 ON CONFLICT (account) DO UPDATE SET micro_usdc = mock_chain_balances.micro_usdc + EXCLUDED.micro_usdc, updated_at = now()`,
		account, amount)
	if err != nil {
		return fmt.Errorf("chain mock: crédito em %s: %w", account, err)
	}
	return nil
}

func debit(ctx context.Context, tx *sqlx.Tx, account port.Account, amount domain.MicroUSDC) error {
	res, err := tx.ExecContext(ctx,
		`UPDATE mock_chain_balances SET micro_usdc = micro_usdc - $2, updated_at = now()
		  WHERE account = $1 AND micro_usdc >= $2`, account, amount)
	if err != nil {
		return fmt.Errorf("chain mock: débito em %s: %w", account, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: conta %s, valor %s", ErrInsufficientFunds, account, amount)
	}
	return nil
}

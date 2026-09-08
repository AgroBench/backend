package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/apperrors"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/internal/wallet/domain"
)

type row struct {
	ID            uuid.UUID  `db:"id"`
	UserID        uuid.UUID  `db:"user_id"`
	Pubkey        string     `db:"pubkey"`
	EncryptedBlob []byte     `db:"encrypted_blob"`
	BlobVersion   int        `db:"blob_version"`
	ExportedAt    *time.Time `db:"exported_at"`
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
}

func (r row) toDomain() domain.Wallet {
	return domain.Wallet{
		ID: r.ID, UserID: r.UserID, Pubkey: r.Pubkey,
		EncryptedBlob: r.EncryptedBlob, BlobVersion: r.BlobVersion,
		ExportedAt: r.ExportedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

type WalletRepo struct{ db sqlx.ExtContext }

func New(db sqlx.ExtContext) *WalletRepo { return &WalletRepo{db: db} }

func (r *WalletRepo) Create(ctx context.Context, w domain.Wallet) error {
	const op = "wallet.Create"
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO wallets (id, user_id, pubkey, encrypted_blob, blob_version)
		VALUES ($1, $2, $3, $4, $5)`,
		w.ID, w.UserID, w.Pubkey, w.EncryptedBlob, w.BlobVersion)
	if err != nil {
		return apperrors.FromDBError(op, err)
	}
	return nil
}

func (r *WalletRepo) GetByUserID(ctx context.Context, userID uuid.UUID) (domain.Wallet, error) {
	return r.get(ctx, "wallet.GetByUserID", `SELECT id, user_id, pubkey, encrypted_blob, blob_version, exported_at, created_at, updated_at FROM wallets WHERE user_id = $1`, userID)
}

func (r *WalletRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Wallet, error) {
	return r.get(ctx, "wallet.GetByID", `SELECT id, user_id, pubkey, encrypted_blob, blob_version, exported_at, created_at, updated_at FROM wallets WHERE id = $1`, id)
}

func (r *WalletRepo) GetPubkeyByUserID(ctx context.Context, userID uuid.UUID) (string, error) {
	w, err := r.GetByUserID(ctx, userID)
	if err != nil {
		return "", err
	}
	return w.Pubkey, nil
}

func (r *WalletRepo) MarkExported(ctx context.Context, id uuid.UUID) error {
	const op = "wallet.MarkExported"
	_, err := r.db.ExecContext(ctx, `UPDATE wallets SET exported_at = now() WHERE id = $1`, id)
	if err != nil {
		return apperrors.FromDBError(op, err)
	}
	return nil
}

func (r *WalletRepo) SumRewards(ctx context.Context, walletID uuid.UUID) (coredomain.MicroUSDC, error) {
	var v float64
	err := sqlx.GetContext(ctx, r.db, &v, `
		SELECT COALESCE(SUM(a.reward_usdc), 0)
		  FROM attestations a
		  JOIN contributions c ON c.id = a.contribution_id
		 WHERE c.wallet_id = $1`, walletID)
	if err != nil {
		return 0, nil
	}
	return coredomain.USDC(v), nil
}

func (r *WalletRepo) get(ctx context.Context, op, q string, arg any) (domain.Wallet, error) {
	var rec row
	err := sqlx.GetContext(ctx, r.db, &rec, q, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Wallet{}, apperrors.NotFound(op, err).WithDetail("wallet não encontrada")
	}
	if err != nil {
		return domain.Wallet{}, apperrors.FromDBError(op, err)
	}
	return rec.toDomain(), nil
}

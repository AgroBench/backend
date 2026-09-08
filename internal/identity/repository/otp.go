package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/apperrors"
	"github.com/AgroBench/backend/internal/identity/domain"
)

type otpRow struct {
	ID         uuid.UUID  `db:"id"`
	UserID     uuid.UUID  `db:"user_id"`
	Purpose    string     `db:"purpose"`
	CodeHash   string     `db:"code_hash"`
	ExpiresAt  time.Time  `db:"expires_at"`
	ConsumedAt *time.Time `db:"consumed_at"`
	Attempts   int        `db:"attempts"`
	CreatedAt  time.Time  `db:"created_at"`
}

func (r otpRow) toDomain() domain.OTPCode {
	return domain.OTPCode{
		ID:         r.ID,
		UserID:     r.UserID,
		Purpose:    domain.OTPPurpose(r.Purpose),
		CodeHash:   r.CodeHash,
		ExpiresAt:  r.ExpiresAt,
		ConsumedAt: r.ConsumedAt,
		Attempts:   r.Attempts,
		CreatedAt:  r.CreatedAt,
	}
}

type OTPRepo struct{ db sqlx.ExtContext }

func NewOTPRepo(db sqlx.ExtContext) *OTPRepo { return &OTPRepo{db: db} }

func (r *OTPRepo) Create(ctx context.Context, o domain.OTPCode) error {
	const op = "identity.OTPRepo.Create"
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO otp_codes (id, user_id, purpose, code_hash, expires_at, attempts)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		o.ID, o.UserID, o.Purpose, o.CodeHash, o.ExpiresAt, o.Attempts)
	if err != nil {
		return apperrors.FromDBError(op, err)
	}
	return nil
}

func (r *OTPRepo) InvalidatePending(ctx context.Context, userID uuid.UUID, purpose domain.OTPPurpose) error {
	const op = "identity.OTPRepo.InvalidatePending"
	_, err := r.db.ExecContext(ctx, `
		UPDATE otp_codes SET consumed_at = now()
		 WHERE user_id = $1 AND purpose = $2 AND consumed_at IS NULL`, userID, purpose)
	if err != nil {
		return apperrors.FromDBError(op, err)
	}
	return nil
}

func (r *OTPRepo) GetPending(ctx context.Context, userID uuid.UUID, purpose domain.OTPPurpose) (domain.OTPCode, error) {
	const op = "identity.OTPRepo.GetPending"
	var row otpRow
	err := sqlx.GetContext(ctx, r.db, &row, `
		SELECT id, user_id, purpose, code_hash, expires_at, consumed_at, attempts, created_at
		  FROM otp_codes
		 WHERE user_id = $1 AND purpose = $2 AND consumed_at IS NULL
		 ORDER BY created_at DESC
		 LIMIT 1`, userID, purpose)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.OTPCode{}, apperrors.NotFound(op, err).WithDetail("código não encontrado")
	}
	if err != nil {
		return domain.OTPCode{}, apperrors.FromDBError(op, err)
	}
	return row.toDomain(), nil
}

func (r *OTPRepo) IncrementAttempts(ctx context.Context, id uuid.UUID) error {
	const op = "identity.OTPRepo.IncrementAttempts"
	_, err := r.db.ExecContext(ctx, `UPDATE otp_codes SET attempts = attempts + 1 WHERE id = $1`, id)
	if err != nil {
		return apperrors.FromDBError(op, err)
	}
	return nil
}

func (r *OTPRepo) Consume(ctx context.Context, id uuid.UUID) error {
	const op = "identity.OTPRepo.Consume"
	_, err := r.db.ExecContext(ctx, `UPDATE otp_codes SET consumed_at = now() WHERE id = $1`, id)
	if err != nil {
		return apperrors.FromDBError(op, err)
	}
	return nil
}

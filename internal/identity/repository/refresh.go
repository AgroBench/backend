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

type refreshRow struct {
	ID        uuid.UUID  `db:"id"`
	UserID    uuid.UUID  `db:"user_id"`
	TokenHash string     `db:"token_hash"`
	ExpiresAt time.Time  `db:"expires_at"`
	RevokedAt *time.Time `db:"revoked_at"`
	Device    *string    `db:"device"`
	CreatedAt time.Time  `db:"created_at"`
}

func (r refreshRow) toDomain() domain.RefreshToken {
	dev := ""
	if r.Device != nil {
		dev = *r.Device
	}
	return domain.RefreshToken{
		ID:        r.ID,
		UserID:    r.UserID,
		TokenHash: r.TokenHash,
		ExpiresAt: r.ExpiresAt,
		RevokedAt: r.RevokedAt,
		Device:    dev,
		CreatedAt: r.CreatedAt,
	}
}

type RefreshRepo struct{ db sqlx.ExtContext }

func NewRefreshRepo(db sqlx.ExtContext) *RefreshRepo { return &RefreshRepo{db: db} }

func (r *RefreshRepo) Create(ctx context.Context, t domain.RefreshToken) error {
	const op = "identity.RefreshRepo.Create"
	var device any
	if t.Device != "" {
		device = t.Device
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, device)
		VALUES ($1, $2, $3, $4, $5)`,
		t.ID, t.UserID, t.TokenHash, t.ExpiresAt, device)
	if err != nil {
		return apperrors.FromDBError(op, err)
	}
	return nil
}

func (r *RefreshRepo) GetByHash(ctx context.Context, hash string) (domain.RefreshToken, error) {
	const op = "identity.RefreshRepo.GetByHash"
	var row refreshRow
	err := sqlx.GetContext(ctx, r.db, &row, `
		SELECT id, user_id, token_hash, expires_at, revoked_at, device, created_at
		  FROM refresh_tokens WHERE token_hash = $1`, hash)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.RefreshToken{}, apperrors.NotFound(op, err).WithDetail("refresh token inválido")
	}
	if err != nil {
		return domain.RefreshToken{}, apperrors.FromDBError(op, err)
	}
	return row.toDomain(), nil
}

func (r *RefreshRepo) Revoke(ctx context.Context, id uuid.UUID) error {
	const op = "identity.RefreshRepo.Revoke"
	_, err := r.db.ExecContext(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, id)
	if err != nil {
		return apperrors.FromDBError(op, err)
	}
	return nil
}

func (r *RefreshRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	const op = "identity.RefreshRepo.RevokeAllForUser"
	_, err := r.db.ExecContext(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	if err != nil {
		return apperrors.FromDBError(op, err)
	}
	return nil
}

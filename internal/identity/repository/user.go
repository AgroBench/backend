package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/apperrors"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/internal/identity/domain"
)

const userCols = `id, email, phone, cpf_hmac, password_hash, role, mfa_enabled, status, created_at, updated_at`

type userRow struct {
	ID           uuid.UUID `db:"id"`
	Email        string    `db:"email"`
	Phone        string    `db:"phone"`
	CPFHMAC      string    `db:"cpf_hmac"`
	PasswordHash string    `db:"password_hash"`
	Role         string    `db:"role"`
	MFAEnabled   bool      `db:"mfa_enabled"`
	Status       string    `db:"status"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

func (r userRow) toDomain() domain.User {
	return domain.User{
		ID:           r.ID,
		Email:        r.Email,
		Phone:        r.Phone,
		CPFHMAC:      r.CPFHMAC,
		PasswordHash: r.PasswordHash,
		Role:         coredomain.Role(r.Role),
		MFAEnabled:   r.MFAEnabled,
		Status:       domain.UserStatus(r.Status),
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

type UserRepo struct{ db sqlx.ExtContext }

func NewUserRepo(db sqlx.ExtContext) *UserRepo { return &UserRepo{db: db} }

func (r *UserRepo) Create(ctx context.Context, u domain.User) error {
	const op = "identity.UserRepo.Create"
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO users (id, email, phone, cpf_hmac, password_hash, role, mfa_enabled, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		u.ID, strings.ToLower(u.Email), u.Phone, u.CPFHMAC, u.PasswordHash, u.Role, u.MFAEnabled, u.Status)
	if err != nil {
		return apperrors.FromDBError(op, err)
	}
	return nil
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	return r.get(ctx, "identity.UserRepo.GetByID", `SELECT `+userCols+` FROM users WHERE id = $1`, id)
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	return r.get(ctx, "identity.UserRepo.GetByEmail", `SELECT `+userCols+` FROM users WHERE email = $1`, strings.ToLower(email))
}

func (r *UserRepo) GetByCPFHMAC(ctx context.Context, hmac string) (domain.User, error) {
	return r.get(ctx, "identity.UserRepo.GetByCPFHMAC", `SELECT `+userCols+` FROM users WHERE cpf_hmac = $1`, hmac)
}

func (r *UserRepo) UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	const op = "identity.UserRepo.UpdatePassword"
	res, err := r.db.ExecContext(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, userID, passwordHash)
	if err != nil {
		return apperrors.FromDBError(op, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return apperrors.NotFound(op, errors.New("user not found"))
	}
	return nil
}

func (r *UserRepo) get(ctx context.Context, op, q string, arg any) (domain.User, error) {
	var row userRow
	err := sqlx.GetContext(ctx, r.db, &row, q, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, apperrors.NotFound(op, err).WithDetail("usuário não encontrado")
	}
	if err != nil {
		return domain.User{}, apperrors.FromDBError(op, err)
	}
	return row.toDomain(), nil
}

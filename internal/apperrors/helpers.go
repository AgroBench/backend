package apperrors

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

func Validation(op string, err error) *AppError { return New(ErrInvalidInput, op, "usecase", err) }
func NotFound(op string, err error) *AppError   { return New(ErrNotFound, op, "repository", err) }
func Conflict(op string, err error) *AppError   { return New(ErrConflict, op, "repository", err) }
func Unauthorized(op string, err error) *AppError {
	return New(ErrUnauthorized, op, "handler", err)
}
func Forbidden(op string, err error) *AppError { return New(ErrForbidden, op, "usecase", err) }
func Internal(op string, err error) *AppError  { return New(ErrInternal, op, "repository", err) }
func ConfigMissing(op string, err error) *AppError {
	return New(ErrConfigMissing, op, "config", err)
}

func External(op string, externalStatus int, err error) *AppError {
	e := New(ErrExternalDependency, op, "gateway", err)
	e.ExternalStatus = externalStatus
	return e
}

// FromDBError mapeia códigos de erro do PostgreSQL para AppError.
func FromDBError(op string, err error) *AppError {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503", "23505", "23514": // foreign_key, unique, check
			return Conflict(op, err)
		case "23502": // not_null
			return Validation(op, err)
		}
	}
	return Internal(op, err)
}

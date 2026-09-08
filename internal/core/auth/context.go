package auth

import (
	"context"

	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/core/domain"
)

type ctxKey struct{}

func WithClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}

func ClaimsFrom(ctx context.Context) *Claims {
	c, _ := ctx.Value(ctxKey{}).(*Claims)
	return c
}

func UserID(ctx context.Context) uuid.UUID {
	c := ClaimsFrom(ctx)
	if c == nil {
		return uuid.Nil
	}
	return c.UserID()
}

func Role(ctx context.Context) domain.Role {
	c := ClaimsFrom(ctx)
	if c == nil {
		return ""
	}
	return c.RoleValue()
}

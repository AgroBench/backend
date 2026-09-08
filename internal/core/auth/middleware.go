package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/AgroBench/backend/internal/apperrors"
	"github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/internal/core/httpx"
)

// Require exige Bearer JWT válido. Se roles for informado, o claim precisa bater.
func Require(roles ...domain.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			const op = "auth.Require"
			raw := r.Header.Get("Authorization")
			token, ok := strings.CutPrefix(raw, "Bearer ")
			if !ok || strings.TrimSpace(token) == "" {
				httpx.Error(w, r, apperrors.Unauthorized(op, errors.New("missing token")).WithDetail("token ausente"))
				return
			}
			claims, err := ParseAccess(token)
			if err != nil {
				detail := "token inválido"
				if errors.Is(err, ErrTokenExpired) {
					detail = "token expirado"
				}
				httpx.Error(w, r, apperrors.Unauthorized(op, err).WithDetail(detail))
				return
			}
			if len(roles) > 0 && !hasRole(claims.RoleValue(), roles) {
				httpx.Error(w, r, apperrors.Forbidden(op, errors.New("role")).WithDetail("perfil insuficiente"))
				return
			}
			next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
		})
	}
}

func hasRole(got domain.Role, allowed []domain.Role) bool {
	for _, r := range allowed {
		if got == r {
			return true
		}
	}
	return false
}

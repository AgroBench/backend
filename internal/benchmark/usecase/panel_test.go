package usecase

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/AgroBench/backend/internal/apperrors"
)

func TestNewPanelBlocked_CarriesConsecutiveCount(t *testing.T) {
	err := NewPanelBlocked(2, 3)

	require.Equal(t, 2, err.CyclesValidated)
	require.Equal(t, 3, err.CyclesRequired)
	require.True(t, apperrors.Is(err, apperrors.ErrForbidden))

	var app *apperrors.AppError
	require.True(t, errors.As(err, &app))
	require.Equal(t, "painel bloqueado: 2/3 ciclos consecutivos", app.Detail)
}

func TestInsertReportAccessSQL_UsesInstitutionIDNotUserID(t *testing.T) {
	sql := strings.Join(strings.Fields(insertReportAccessSQL), " ")
	require.Contains(t, sql, "SELECT $1, i.id, s.id, $3")
	require.Contains(t, sql, "FROM institutions i")
	require.Contains(t, sql, "JOIN subscriptions s ON s.institution_id = i.id")
	require.Contains(t, sql, "WHERE i.user_id = $2")
	require.NotContains(t, sql, "s.institution_id = $2")
}

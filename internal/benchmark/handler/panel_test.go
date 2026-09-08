package handler

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/AgroBench/backend/internal/apperrors"
	"github.com/AgroBench/backend/internal/benchmark/usecase"
)

func TestPanelForbiddenJSON_UsesRealCyclesValidated(t *testing.T) {
	viper.Set("benchmark.free_after_cycles", 3)

	for _, n := range []int{0, 1, 2} {
		body := panelForbiddenJSON(usecase.NewPanelBlocked(n, 3))
		require.Equal(t, "FORBIDDEN", body["code"])
		require.Equal(t, "Painel bloqueado", body["message"])
		require.Equal(t, n, body["cycles_validated"])
		require.Equal(t, 3, body["cycles_required"])
		require.Contains(t, body["detail"], "painel bloqueado:")

		raw, err := json.Marshal(body)
		require.NoError(t, err)
		require.Contains(t, string(raw), `"cycles_validated":`)
	}
}

func TestPanelForbiddenJSON_PlainForbiddenKeepsZero(t *testing.T) {
	viper.Set("benchmark.free_after_cycles", 3)
	err := apperrors.Forbidden("benchmark.Me", errors.New("not enough")).
		WithDetail("painel bloqueado: 1/3 ciclos consecutivos")

	body := panelForbiddenJSON(err)
	require.Equal(t, 0, body["cycles_validated"])
	require.Equal(t, 3, body["cycles_required"])
	require.Equal(t, "painel bloqueado: 1/3 ciclos consecutivos", body["detail"])
}

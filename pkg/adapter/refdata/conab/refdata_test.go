package conab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/AgroBench/backend/internal/core/payload"
)

func TestExpectedRangesConvertsMeanToBand(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "soybean", r.URL.Query().Get("cultura"))
		require.Equal(t, "43017", r.URL.Query().Get("microrregiao"))
		_ = json.NewEncoder(w).Encode(apiResponse{
			CustoTotalHa:      5000,
			CustoFertilizante: 1000,
			ProdutividadeScHa: 50,
		})
	}))
	t.Cleanup(ts.Close)

	c, err := New(Config{BaseURL: ts.URL, TolerancePct: 40})
	require.NoError(t, err)

	ranges, err := c.ExpectedRanges(context.Background(), "soybean", "43017")
	require.NoError(t, err)
	by := map[string][2]float64{}
	for _, r := range ranges {
		by[r.Metric] = [2]float64{r.Min, r.Max}
	}
	require.Equal(t, [2]float64{3000, 7000}, by[payload.MetricTotalCostHa])
	require.Equal(t, [2]float64{600, 1400}, by[payload.MetricFertilizerCostHa])
	require.Equal(t, [2]float64{30, 70}, by[payload.MetricYieldSacksHa])
	_, hasPesticide := by[payload.MetricPesticideCostHa]
	require.False(t, hasPesticide, "média zero não vira faixa")
}

func TestExpectedRangesNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(ts.Close)

	c, err := New(Config{BaseURL: ts.URL})
	require.NoError(t, err)
	ranges, err := c.ExpectedRanges(context.Background(), "wheat", "00000")
	require.NoError(t, err)
	require.Nil(t, ranges)
}

func TestNewRequiresBaseURL(t *testing.T) {
	_, err := New(Config{})
	require.Error(t, err)
}

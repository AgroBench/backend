package payload_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/internal/core/payload"
)

const nonce = "0123456789abcdef0123456789abcdef"

func TestBuildParseBasic(t *testing.T) {
	cycle := uuid.New()
	raw, err := payload.Build(domain.LevelBasic, cycle, nonce, payload.Basic{CultureCode: "soybean", AreaHa: 50, TotalCostBRL: 250_000})
	require.NoError(t, err)

	p, err := payload.Parse(raw)
	require.NoError(t, err)
	require.Equal(t, cycle, p.Envelope.CycleID)
	require.NotNil(t, p.Basic)
	require.Nil(t, p.Inter)

	m := metricsMap(p)
	require.InDelta(t, 5000, m[payload.MetricTotalCostHa], 0.001)
	require.Len(t, p.Metrics(), 2)
}

func TestAdvancedMetrics(t *testing.T) {
	adv := payload.Advanced{
		Intermediate: payload.Intermediate{
			Basic:        payload.Basic{CultureCode: "soybean", AreaHa: 100, TotalCostBRL: 500_000},
			CostByInput:  payload.CostByInput{FertilizerBRL: 120_000, PesticideBRL: 80_000, SeedBRL: 50_000, FuelBRL: 30_000, LaborBRL: 40_000},
			YieldSacksHa: 70,
			PlantingDate: "2025-10-15",
			HarvestDate:  "2026-03-01",
		},
		SoilType:        "latossolo",
		RotationHistory: []string{"corn", "wheat"},
		Irrigation:      payload.Irrigation{Used: true, System: "center_pivot"},
		ClimateLosses:   []payload.ClimateLoss{{Event: "drought", AreaPct: 12.5}, {Event: "hail", AreaPct: 5}},
		Mechanization:   payload.Mechanization{Type: "own", Machinery: []string{"colheitadeira"}},
	}
	raw, err := payload.Build(domain.LevelAdvanced, uuid.New(), nonce, adv)
	require.NoError(t, err)
	p, err := payload.Parse(raw)
	require.NoError(t, err)

	m := metricsMap(p)
	require.InDelta(t, 1200, m[payload.MetricFertilizerCostHa], 0.001)
	require.InDelta(t, 137, m[payload.MetricCycleDays], 0.001)
	require.Equal(t, 1.0, m[payload.MetricIrrigation])
	require.InDelta(t, 17.5, m[payload.MetricClimateLossPct], 0.001)
	require.Equal(t, 1.0, m[payload.MetricMechanizationOwn])
}

func TestHashIsOverExactBytes(t *testing.T) {
	a := []byte(`{"version":1,"level":"basic"}`)
	b := []byte(`{"version": 1, "level": "basic"}`)
	require.NotEqual(t, payload.Hash(a), payload.Hash(b), "bytes diferentes → hash diferente (sem canonicalização implícita)")

	ca, _ := payload.Canonicalize(a)
	cb, _ := payload.Canonicalize(b)
	require.Equal(t, payload.Hash(ca), payload.Hash(cb), "após Canonicalize, o hash bate")
}

func TestRejectsBadPayloads(t *testing.T) {
	cycle := uuid.New()
	cases := map[string][]byte{
		"nonce curto":      mustBuild(t, domain.LevelBasic, cycle, "abcd", payload.Basic{CultureCode: "soybean", AreaHa: 1, TotalCostBRL: 1}),
		"area zero":        mustBuild(t, domain.LevelBasic, cycle, nonce, payload.Basic{CultureCode: "soybean", AreaHa: 0, TotalCostBRL: 1}),
		"nível sem campos": mustBuild(t, domain.LevelIntermediate, cycle, nonce, payload.Basic{CultureCode: "soybean", AreaHa: 1, TotalCostBRL: 1}),
		"json inválido":    []byte(`{"version":1,`),
	}
	for name, raw := range cases {
		_, err := payload.Parse(raw)
		require.Error(t, err, name)
	}
}

func mustBuild(t *testing.T, level domain.ContributionLevel, cycle uuid.UUID, nonce string, data any) []byte {
	t.Helper()
	raw, err := payload.Build(level, cycle, nonce, data)
	require.NoError(t, err)
	return raw
}

func metricsMap(p *payload.Parsed) map[string]float64 {
	out := map[string]float64{}
	for _, m := range p.Metrics() {
		out[m.Name] = m.Value
	}
	return out
}

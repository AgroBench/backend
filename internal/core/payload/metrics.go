package payload

import (
	"bytes"
	"io"
	"time"
)

// Nomes das métricas agregáveis. São o que sai do enclave e o que o painel mostra.
// Custos são em BRL por hectare; produtividade em sacas por hectare.
const (
	MetricAreaHa           = "area_ha"
	MetricTotalCostHa      = "total_cost_ha"
	MetricFertilizerCostHa = "fertilizer_cost_ha"
	MetricPesticideCostHa  = "pesticide_cost_ha"
	MetricSeedCostHa       = "seed_cost_ha"
	MetricFuelCostHa       = "fuel_cost_ha"
	MetricLaborCostHa      = "labor_cost_ha"
	MetricYieldSacksHa     = "yield_sacks_ha"
	MetricCycleDays        = "cycle_days"
	MetricIrrigation       = "irrigation"        // 0 ou 1
	MetricClimateLossPct   = "climate_loss_pct"  // soma das perdas
	MetricPestEvents       = "pest_events"       // contagem
	MetricMechanizationOwn = "mechanization_own" // 0 ou 1
)

// BasicMetrics são as únicas que aparecem no painel gratuito do produtor (README raiz §6.3).
var BasicMetrics = []string{MetricAreaHa, MetricTotalCostHa}

type Metric struct {
	Name  string
	Value float64
}

// Metrics deriva as métricas numéricas anonimizadas do payload, conforme o nível.
// Fornecedores, tipo de solo e histórico de rotação NÃO viram métrica numérica aqui;
// entram em agregações categóricas futuras e nunca saem individualmente.
func (p *Parsed) Metrics() []Metric {
	b := p.Basic
	out := []Metric{
		{MetricAreaHa, b.AreaHa},
		{MetricTotalCostHa, b.TotalCostBRL / b.AreaHa},
	}
	if p.Inter != nil {
		i := p.Inter
		out = append(out,
			Metric{MetricFertilizerCostHa, i.CostByInput.FertilizerBRL / b.AreaHa},
			Metric{MetricPesticideCostHa, i.CostByInput.PesticideBRL / b.AreaHa},
			Metric{MetricSeedCostHa, i.CostByInput.SeedBRL / b.AreaHa},
			Metric{MetricFuelCostHa, i.CostByInput.FuelBRL / b.AreaHa},
			Metric{MetricLaborCostHa, i.CostByInput.LaborBRL / b.AreaHa},
			Metric{MetricYieldSacksHa, i.YieldSacksHa},
		)
		if pd, err := time.Parse("2006-01-02", i.PlantingDate); err == nil {
			if hd, err := time.Parse("2006-01-02", i.HarvestDate); err == nil {
				out = append(out, Metric{MetricCycleDays, hd.Sub(pd).Hours() / 24})
			}
		}
	}
	if p.Adv != nil {
		a := p.Adv
		var loss float64
		for _, l := range a.ClimateLosses {
			loss += l.AreaPct
		}
		out = append(out,
			Metric{MetricIrrigation, boolMetric(a.Irrigation.Used)},
			Metric{MetricClimateLossPct, min(loss, 100)},
			Metric{MetricPestEvents, float64(len(a.PestEvents))},
			Metric{MetricMechanizationOwn, boolMetric(a.Mechanization.Type == "own")},
		)
	}
	return out
}

func boolMetric(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

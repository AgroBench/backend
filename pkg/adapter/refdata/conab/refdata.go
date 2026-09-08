// Package conab é a implementação REAL de port.ReferenceDataClient (NÃO ativa no pitch).
//
// TODO(conab): a CONAB publica custos de produção e séries históricas de produtividade em
// planilhas (https://www.conab.gov.br/info-agro/custos-de-producao) e no Portal de
// Informações Agropecuárias; não há API JSON pública confirmada por cultura × microrregião.
// Este adapter deixa pronto o client, a montagem da URL, o mapeamento e a conversão de
// faixa (média ± tolerância). Ajuste `pathTemplate` e `apiResponse` quando a fonte for
// definida (pode virar um importador de planilha alimentando uma tabela local).
package conab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/AgroBench/backend/internal/core/payload"
	"github.com/AgroBench/backend/pkg/port"
)

type Config struct {
	BaseURL      string
	Timeout      time.Duration
	TolerancePct float64 // faixa = média × (1 ± tolerância). Default 40%.
}

type Client struct {
	cfg  Config
	http *http.Client
}

// TODO(conab): confirmar path e parâmetros.
const pathTemplate = "/api/v1/custos?cultura=%s&microrregiao=%s"

// apiResponse é o formato ASSUMIDO. Valores em BRL/ha e sacas/ha.
type apiResponse struct {
	CustoTotalHa      float64 `json:"custo_total_ha"`
	CustoFertilizante float64 `json:"custo_fertilizante_ha"`
	CustoDefensivo    float64 `json:"custo_defensivo_ha"`
	CustoSemente      float64 `json:"custo_semente_ha"`
	ProdutividadeScHa float64 `json:"produtividade_sc_ha"`
}

func New(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, errors.New("refdata conab: base_url é obrigatória")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.TolerancePct == 0 {
		cfg.TolerancePct = 40
	}
	return &Client{cfg: cfg, http: &http.Client{Timeout: cfg.Timeout}}, nil
}

func (c *Client) ExpectedRanges(ctx context.Context, cultureCode, ibgeCode string) ([]port.ReferenceRange, error) {
	url := strings.TrimRight(c.cfg.BaseURL, "/") + fmt.Sprintf(pathTemplate, cultureCode, ibgeCode)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refdata conab: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil, nil // sem referência para essa combinação
	}
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("refdata conab: status %d", res.StatusCode)
	}

	var body apiResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("refdata conab: resposta inválida: %w", err)
	}

	var out []port.ReferenceRange
	add := func(metric string, mean float64) {
		if mean <= 0 {
			return
		}
		t := c.cfg.TolerancePct / 100
		out = append(out, port.ReferenceRange{Metric: metric, Min: mean * (1 - t), Max: mean * (1 + t)})
	}
	add(payload.MetricTotalCostHa, body.CustoTotalHa)
	add(payload.MetricFertilizerCostHa, body.CustoFertilizante)
	add(payload.MetricPesticideCostHa, body.CustoDefensivo)
	add(payload.MetricSeedCostHa, body.CustoSemente)
	add(payload.MetricYieldSacksHa, body.ProdutividadeScHa)
	return out, nil
}

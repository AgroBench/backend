// Package http é a implementação REAL de port.SicarClient (NÃO ativa no pitch).
//
// TODO(sicar): o endpoint público de consulta do SICAR não foi confirmado dentro do escopo
// do MVP. A consulta pública conhecida é por interface web (https://consultapublica.car.gov.br)
// e há endpoints de download de shapefile por estado; não há garantia de API JSON por
// número de CAR. Este adapter deixa pronto: client HTTP com timeout, montagem da URL,
// mapeamento de resposta e tratamento de erro. Ajuste `pathTemplate` e `apiResponse`
// quando o endpoint for confirmado.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdhttp "net/http"
	"strings"
	"time"

	"github.com/AgroBench/backend/internal/core/crypto"
	"github.com/AgroBench/backend/pkg/port"
)

type Config struct {
	BaseURL string        // ex.: https://api.car.gov.br  (TODO: confirmar)
	APIKey  string        // se o endpoint exigir
	Timeout time.Duration // default 10s
}

type Client struct {
	cfg  Config
	http *stdhttp.Client
}

// TODO(sicar): confirmar o path real. Placeholder segue o padrão REST mais provável.
const pathTemplate = "/api/v1/imoveis/%s"

// apiResponse é o formato ASSUMIDO da resposta. TODO(sicar): ajustar aos campos reais.
type apiResponse struct {
	Numero    string `json:"numero"`
	Situacao  string `json:"situacao"` // "AT" ativo, "PE" pendente, "CA" cancelado, "SU" suspenso
	UF        string `json:"uf"`
	Municipio struct {
		CodigoIBGE string `json:"codigo_ibge"`
	} `json:"municipio"`
}

func New(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, errors.New("sicar http: base_url é obrigatória")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &Client{cfg: cfg, http: &stdhttp.Client{Timeout: cfg.Timeout}}, nil
}

func (c *Client) Lookup(ctx context.Context, car string) (port.CARRecord, error) {
	url := strings.TrimRight(c.cfg.BaseURL, "/") + fmt.Sprintf(pathTemplate, crypto.NormalizeIdentifier(car))
	req, err := stdhttp.NewRequestWithContext(ctx, stdhttp.MethodGet, url, nil)
	if err != nil {
		return port.CARRecord{}, err
	}
	req.Header.Set("Accept", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return port.CARRecord{}, fmt.Errorf("sicar http: %w", err)
	}
	defer res.Body.Close()

	switch {
	case res.StatusCode == stdhttp.StatusNotFound:
		return port.CARRecord{Exists: false}, nil
	case res.StatusCode >= 400:
		return port.CARRecord{}, fmt.Errorf("sicar http: status %d", res.StatusCode)
	}

	var body apiResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return port.CARRecord{}, fmt.Errorf("sicar http: resposta inválida: %w", err)
	}
	return port.CARRecord{
		Exists:   true,
		Active:   strings.EqualFold(body.Situacao, "AT"),
		UF:       body.UF,
		IBGECode: body.Municipio.CodigoIBGE,
	}, nil
}

// Package mock é a implementação de port.ReferenceDataClient ATIVA NO PITCH: lê as faixas
// da tabela seed mock_reference_ranges em vez das séries da CONAB.
package mock

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/pkg/port"
)

type Client struct{ db *sqlx.DB }

func New(db *sqlx.DB) *Client { return &Client{db: db} }

func (c *Client) ExpectedRanges(ctx context.Context, cultureCode, ibgeCode string) ([]port.ReferenceRange, error) {
	var rows []struct {
		Metric string  `db:"metric"`
		Min    float64 `db:"min_value"`
		Max    float64 `db:"max_value"`
	}
	err := c.db.SelectContext(ctx, &rows,
		`SELECT metric, min_value, max_value FROM mock_reference_ranges
		  WHERE culture_code = $1 AND ibge_code = $2 ORDER BY metric`, cultureCode, ibgeCode)
	if err != nil {
		return nil, fmt.Errorf("refdata mock: %w", err)
	}
	out := make([]port.ReferenceRange, len(rows))
	for i, r := range rows {
		out[i] = port.ReferenceRange{Metric: r.Metric, Min: r.Min, Max: r.Max}
	}
	return out, nil
}

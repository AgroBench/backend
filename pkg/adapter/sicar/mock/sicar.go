// Package mock é a implementação de port.SicarClient ATIVA NO PITCH: consulta a tabela
// seed mock_sicar_cars em vez da base pública do SICAR.
package mock

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/core/crypto"
	"github.com/AgroBench/backend/pkg/port"
)

type Client struct{ db *sqlx.DB }

func New(db *sqlx.DB) *Client { return &Client{db: db} }

func (c *Client) Lookup(ctx context.Context, car string) (port.CARRecord, error) {
	var row struct {
		Active   bool   `db:"active"`
		UF       string `db:"uf"`
		IBGECode string `db:"ibge_code"`
	}
	err := c.db.GetContext(ctx, &row,
		`SELECT active, uf, ibge_code FROM mock_sicar_cars WHERE car_number = $1`,
		crypto.NormalizeIdentifier(car))
	if errors.Is(err, sql.ErrNoRows) {
		return port.CARRecord{Exists: false}, nil
	}
	if err != nil {
		return port.CARRecord{}, fmt.Errorf("sicar mock: %w", err)
	}
	return port.CARRecord{Exists: true, Active: row.Active, UF: row.UF, IBGECode: row.IBGECode}, nil
}

package seed

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/core/crypto"
)

const passoFundoIBGE = "43010"
const passoFundoMun = "4314100"

func seedMockCars(ctx context.Context, db *sqlx.DB) error {
	for i := 1; i <= 20; i++ {
		raw := fmt.Sprintf("RS-%s-AAA%017d", passoFundoMun, i)
		active := i <= 17
		_, err := db.ExecContext(ctx, `
			INSERT INTO mock_sicar_cars (car_number, active, uf, ibge_code)
			VALUES ($1, $2, 'RS', $3)
			ON CONFLICT (car_number) DO NOTHING`,
			crypto.NormalizeIdentifier(raw), active, passoFundoMun)
		if err != nil {
			return fmt.Errorf("mock car %d: %w", i, err)
		}
	}
	return nil
}

func seedMockRanges(ctx context.Context, db *sqlx.DB) error {
	type rng struct {
		culture, metric string
		min, max        float64
	}
	ranges := []rng{
		{"soybean", "total_cost_ha", 2000, 8000},
		{"soybean", "yield_sacks_ha", 20, 90},
		{"soybean", "area_ha", 1, 5000},
		{"soybean", "fertilizer_cost_ha", 200, 3000},
		{"corn", "total_cost_ha", 1800, 7000},
		{"corn", "yield_sacks_ha", 40, 180},
		{"corn", "area_ha", 1, 5000},
		{"wheat", "total_cost_ha", 1500, 6000},
		{"wheat", "yield_sacks_ha", 15, 80},
		{"wheat", "area_ha", 1, 5000},
	}
	for _, r := range ranges {
		_, err := db.ExecContext(ctx, `
			INSERT INTO mock_reference_ranges (culture_code, ibge_code, metric, min_value, max_value)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (culture_code, ibge_code, metric) DO NOTHING`,
			r.culture, passoFundoIBGE, r.metric, r.min, r.max)
		if err != nil {
			return err
		}
	}
	return nil
}

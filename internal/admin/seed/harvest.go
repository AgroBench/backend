package seed

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/core/crypto"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
)

type harvestPlanRow struct {
	regionIBGE string
	culture    string
	labels     []string // ciclos aggregated
	openLive   bool     // também abre 2026/27 (produtor oficial contribui ao vivo)
}

var harvestPlan = []harvestPlanRow{
	{passoFundoIBGE, "soybean", []string{"2023/24", "2024/25", "2025/26"}, true},
	{passoFundoIBGE, "corn", []string{"2024/25", "2025/26"}, false},
	{passoFundoIBGE, "wheat", []string{"2025/26"}, false},
	{carazinhoIBGE, "soybean", []string{"2023/24", "2024/25", "2025/26"}, false},
	{naoMeToqueIBGE, "soybean", []string{"2023/24", "2024/25", "2025/26"}, false},
	{ijuiIBGE, "corn", []string{"2024/25", "2025/26"}, false},
	{cruzAltaIBGE, "soybean", []string{"2024/25", "2025/26"}, false},
	{erechimIBGE, "wheat", []string{"2024/25", "2025/26"}, false},
	{santaRosaIBGE, "soybean", []string{"2025/26"}, false},
	{vacariaIBGE, "wheat", []string{"2024/25", "2025/26"}, false},
	{santoAngeloIBGE, "soybean", []string{"2025/26"}, false},
}

const liveCycleLabel = "2026/27"

type cycleKey struct {
	culture, regionIBGE, label string
}

func nowUTC() time.Time { return time.Now().UTC() }

func labelWindow(label string) (opens, closes time.Time) {
	switch label {
	case "2023/24":
		return time.Date(2023, 9, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	case "2024/25":
		return time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC), time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	case "2025/26":
		return time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	case liveCycleLabel:
		return time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC)
	default:
		return time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	}
}

func yearFactor(label string) (cost, yield float64) {
	switch label {
	case "2023/24":
		return 0.93, 0.96
	case "2024/25":
		return 1.00, 1.00
	case "2025/26":
		return 1.05, 1.02
	case liveCycleLabel:
		return 1.07, 1.00
	default:
		return 1, 1
	}
}

func clamp(v, lo, hi float64) float64 {
	return math.Min(hi, math.Max(lo, v))
}

func scaledMetrics(culture, label string, area, cost, yield float64) (a, c, y float64) {
	cf, yf := yearFactor(label)
	a = math.Round(area*10) / 10
	c = math.Round(cost * cf)
	y = math.Round(yield*yf*10) / 10
	switch culture {
	case "corn":
		c = clamp(c, 1800, 7000)
		y = clamp(y, 40, 180)
	case "wheat":
		c = clamp(c, 1500, 6000)
		y = clamp(y, 15, 80)
	default:
		c = clamp(c, 2000, 8000)
		y = clamp(y, 20, 90)
	}
	a = clamp(a, 1, 5000)
	return a, c, y
}

func lookupIDs(ctx context.Context, db *sqlx.DB) (cultures map[string]uuid.UUID, regions map[string]uuid.UUID, err error) {
	cultures = map[string]uuid.UUID{}
	regions = map[string]uuid.UUID{}
	var rows []struct {
		Code string    `db:"code"`
		ID   uuid.UUID `db:"id"`
	}
	if err = db.SelectContext(ctx, &rows, `SELECT code, id FROM cultures`); err != nil {
		return nil, nil, err
	}
	for _, r := range rows {
		cultures[r.Code] = r.ID
	}
	var regs []struct {
		Code string    `db:"ibge_code"`
		ID   uuid.UUID `db:"id"`
	}
	if err = db.SelectContext(ctx, &regs, `SELECT ibge_code, id FROM micro_regions`); err != nil {
		return nil, nil, err
	}
	for _, r := range regs {
		regions[r.Code] = r.ID
	}
	return cultures, regions, nil
}

func ensureHarvestCycles(ctx context.Context, db *sqlx.DB) (map[cycleKey]uuid.UUID, error) {
	cultures, regions, err := lookupIDs(ctx, db)
	if err != nil {
		return nil, err
	}
	ids := map[cycleKey]uuid.UUID{}
	for _, plan := range harvestPlan {
		regionID, ok := regions[plan.regionIBGE]
		if !ok {
			return nil, fmt.Errorf("microrregião %s ausente", plan.regionIBGE)
		}
		cultureID, ok := cultures[plan.culture]
		if !ok {
			return nil, fmt.Errorf("cultura %s ausente", plan.culture)
		}
		labels := append([]string{}, plan.labels...)
		if plan.openLive {
			labels = append(labels, liveCycleLabel)
		}
		for _, label := range labels {
			status := "aggregated"
			if label == liveCycleLabel {
				status = "open"
			}
			opens, closes := labelWindow(label)
			if plan.regionIBGE == passoFundoIBGE && plan.culture == "soybean" && label == "2025/26" {
				opens = opens.Add(10 * 24 * time.Hour)
			}
			cid := coredomain.NewID()
			_, err := db.ExecContext(ctx, `
				INSERT INTO cycles (id, culture_id, micro_region_id, label, opens_at, closes_at, status)
				VALUES ($1,$2,$3,$4,$5,$6,$7)
				ON CONFLICT (culture_id, micro_region_id, label) DO NOTHING`,
				cid, cultureID, regionID, label, opens, closes, status)
			if err != nil {
				return nil, fmt.Errorf("ciclo %s %s %s: %w", plan.culture, plan.regionIBGE, label, err)
			}
			if err := db.GetContext(ctx, &cid, `
				SELECT id FROM cycles WHERE culture_id=$1 AND micro_region_id=$2 AND label=$3`,
				cultureID, regionID, label); err != nil {
				return nil, err
			}
			ids[cycleKey{plan.culture, plan.regionIBGE, label}] = cid
			if label == liveCycleLabel && plan.regionIBGE == passoFundoIBGE && plan.culture == "soybean" {
				_, _ = db.ExecContext(ctx, `
					UPDATE cycles SET status = 'open', opens_at = $2, closes_at = $3
					 WHERE id = $1
					   AND NOT EXISTS (
					     SELECT 1 FROM contributions WHERE cycle_id = $1 AND commit_tx <> 'seed_demo'
					   )`, cid, opens, closes)
			}
		}
	}
	return ids, nil
}

func ensureContribution(ctx context.Context, db *sqlx.DB, cycleID, walletID, propID uuid.UUID, area, cost, yield float64) error {
	hash := crypto.SHA256Hex([]byte(fmt.Sprintf("seed:%s:%s", walletID, cycleID)))
	cid := coredomain.NewID()
	_, err := db.ExecContext(ctx, `
		INSERT INTO contributions (id, cycle_id, wallet_id, property_id, level, attempt, commit_hash, commit_tx, commit_at, status)
		VALUES ($1,$2,$3,$4,'basic',1,$5,'seed_demo', now(), 'accepted')
		ON CONFLICT DO NOTHING`,
		cid, cycleID, walletID, propID, hash)
	if err != nil {
		return err
	}
	if err := db.GetContext(ctx, &cid, `
		SELECT id FROM contributions WHERE cycle_id=$1 AND wallet_id=$2 AND status='accepted'`,
		cycleID, walletID); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM validated_metrics WHERE contribution_id=$1`, cid); err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO validated_metrics (id, contribution_id, metric, value) VALUES
		($1,$2,'area_ha',$3), ($4,$2,'total_cost_ha',$5), ($6,$2,'yield_sacks_ha',$7)`,
		coredomain.NewID(), cid, area,
		coredomain.NewID(), cost,
		coredomain.NewID(), yield)
	return err
}

func seedHarvestContributions(ctx context.Context, db *sqlx.DB, cycles map[cycleKey]uuid.UUID, main producerSeed, extras []seededFarmer) error {
	mainMetrics := map[string][3]float64{
		"2023/24": {48, 4100, 58},
		"2024/25": {50, 4250, 61},
		"2025/26": {52, 4180, 64},
	}
	for label, m := range mainMetrics {
		cid, ok := cycles[cycleKey{"soybean", passoFundoIBGE, label}]
		if !ok || main.walletID == uuid.Nil {
			continue
		}
		if err := ensureContribution(ctx, db, cid, main.walletID, main.propID, m[0], m[1], m[2]); err != nil {
			return fmt.Errorf("contribuição produtor oficial %s: %w", label, err)
		}
	}

	for _, f := range extras {
		for _, plan := range harvestPlan {
			if plan.regionIBGE != f.regionIBGE || plan.culture != f.culture {
				continue
			}
			labels := append([]string{}, plan.labels...)
			if plan.openLive {
				labels = append(labels, liveCycleLabel)
			}
			for _, label := range labels {
				cid, ok := cycles[cycleKey{f.culture, f.regionIBGE, label}]
				if !ok {
					continue
				}
				a, c, y := scaledMetrics(f.culture, label, f.areaHa, f.costHa, f.yieldHa)
				if err := ensureContribution(ctx, db, cid, f.walletID, f.propID, a, c, y); err != nil {
					return fmt.Errorf("contribuição %s %s %s: %w", extraFarmerEmailOf(f.seq), label, f.culture, err)
				}
			}
		}
	}
	return nil
}

func recomputeAggregates(ctx context.Context, db *sqlx.DB, cycles map[cycleKey]uuid.UUID) error {
	seen := map[uuid.UUID]struct{}{}
	for key, cid := range cycles {
		if _, ok := seen[cid]; ok {
			continue
		}
		seen[cid] = struct{}{}
		if key.label == liveCycleLabel {
			// Ciclo aberto: o worker de close insere aggregates sem ON CONFLICT.
			_, _ = db.ExecContext(ctx, `DELETE FROM contribution_weights WHERE cycle_id = $1`, cid)
			_, _ = db.ExecContext(ctx, `DELETE FROM aggregates WHERE cycle_id = $1`, cid)
			continue
		}
		if err := upsertCycleAggregates(ctx, db, cid); err != nil {
			return err
		}
	}
	return nil
}

func upsertCycleAggregates(ctx context.Context, db *sqlx.DB, cycleID uuid.UUID) error {
	type row struct {
		Level  string    `db:"level"`
		Metric string    `db:"metric"`
		Value  float64   `db:"value"`
		Wallet uuid.UUID `db:"wallet_id"`
	}
	var rows []row
	if err := db.SelectContext(ctx, &rows, `
		SELECT c.level, m.metric, m.value, c.wallet_id
		  FROM validated_metrics m
		  JOIN contributions c ON c.id = m.contribution_id
		 WHERE c.cycle_id = $1 AND c.status = 'accepted'`, cycleID); err != nil {
		return err
	}
	type key struct{ level, metric string }
	groups := map[key][]float64{}
	wallets := map[uuid.UUID]struct{}{}
	for _, r := range rows {
		k := key{r.Level, r.Metric}
		groups[k] = append(groups[k], r.Value)
		wallets[r.Wallet] = struct{}{}
	}
	for k, vals := range groups {
		mean, median, p25, p75 := seedStats(vals)
		_, err := db.ExecContext(ctx, `
			INSERT INTO aggregates (id, cycle_id, level, metric, mean, median, p25, p75, n)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (cycle_id, level, metric) DO UPDATE SET
			  mean = EXCLUDED.mean, median = EXCLUDED.median,
			  p25 = EXCLUDED.p25, p75 = EXCLUDED.p75, n = EXCLUDED.n`,
			coredomain.NewID(), cycleID, k.level, k.metric, mean, median, p25, p75, len(vals))
		if err != nil {
			return err
		}
	}
	n := float64(len(wallets))
	if n == 0 {
		return nil
	}
	for wal := range wallets {
		_, err := db.ExecContext(ctx, `
			INSERT INTO contribution_weights (id, cycle_id, wallet_id, weight)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (cycle_id, wallet_id) DO UPDATE SET weight = EXCLUDED.weight`,
			coredomain.NewID(), cycleID, wal, 1/n)
		if err != nil {
			return err
		}
	}
	return nil
}

func seedStats(vals []float64) (mean, median, p25, p75 float64) {
	if len(vals) == 0 {
		return
	}
	sort.Float64s(vals)
	var sum float64
	for _, v := range vals {
		sum += v
	}
	mean = sum / float64(len(vals))
	median = seedPercentile(vals, 50)
	p25 = seedPercentile(vals, 25)
	p75 = seedPercentile(vals, 75)
	return
}

func seedPercentile(sorted []float64, p float64) float64 {
	if len(sorted) == 1 {
		return sorted[0]
	}
	idx := (p / 100) * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	w := idx - float64(lo)
	return sorted[lo]*(1-w) + sorted[hi]*w
}

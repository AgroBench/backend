package seed

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/core/auth"
	"github.com/AgroBench/backend/internal/core/crypto"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	iddomain "github.com/AgroBench/backend/internal/identity/domain"
	idrepo "github.com/AgroBench/backend/internal/identity/repository"
	"github.com/AgroBench/backend/pkg/adapter/registry"
	"github.com/AgroBench/backend/pkg/port"
)

// Demo cria 14 produtores em Passo Fundo × soja, 3 ciclos agregados, 1 instituição e um pool.
func Demo(ctx context.Context, db *sqlx.DB, adapters *registry.Adapters) error {
	if _, err := Base(ctx, db, adapters); err != nil {
		return err
	}
	var regionID, cultureID uuid.UUID
	if err := db.GetContext(ctx, &regionID, `SELECT id FROM micro_regions WHERE ibge_code = $1`, passoFundoIBGE); err != nil {
		return fmt.Errorf("passo fundo: %w", err)
	}
	if err := db.GetContext(ctx, &cultureID, `SELECT id FROM cultures WHERE code = 'soybean'`); err != nil {
		return err
	}

	hash, err := crypto.HashPassword("demo12345")
	if err != nil {
		return err
	}
	users := idrepo.NewUserRepo(db)
	type prod struct {
		userID, walletID, propID uuid.UUID
		pubkey                   string
	}
	prods := make([]prod, 0, 14)
	for i := 1; i <= 14; i++ {
		uid := coredomain.NewID()
		email := fmt.Sprintf("produtor%02d@agrobench.local", i)
		if _, err := users.GetByEmail(ctx, email); err == nil {
			var existing uuid.UUID
			_ = db.GetContext(ctx, &existing, `SELECT id FROM users WHERE email = $1`, email)
			var wal, prop uuid.UUID
			var pk string
			_ = db.GetContext(ctx, &wal, `SELECT id FROM wallets WHERE user_id = $1`, existing)
			_ = db.GetContext(ctx, &pk, `SELECT pubkey FROM wallets WHERE user_id = $1`, existing)
			_ = db.GetContext(ctx, &prop, `SELECT id FROM properties WHERE user_id = $1`, existing)
			prods = append(prods, prod{existing, wal, prop, pk})
			continue
		}
		u := iddomain.User{
			ID: uid, Email: email, Phone: fmt.Sprintf("+555499900%04d", i),
			CPFHMAC:      auth.HMACIdentifier(fmt.Sprintf("%011d", 10000000000+i)),
			PasswordHash: hash, Role: coredomain.RoleProducer, MFAEnabled: true, Status: iddomain.UserActive,
		}
		if err := users.Create(ctx, u); err != nil {
			return err
		}
		wid := coredomain.NewID()
		pk := fmt.Sprintf("DemoWallet%02d%038d", i, i)
		if len(pk) > 44 {
			pk = pk[:44]
		}
		if _, err := db.ExecContext(ctx, `
			INSERT INTO wallets (id, user_id, pubkey, encrypted_blob, blob_version)
			VALUES ($1,$2,$3,$4,1)`, wid, uid, pk, []byte("demo")); err != nil {
			return err
		}
		if adapters != nil && adapters.ChainMock != nil {
			_ = adapters.ChainMock.Mint(ctx, port.Account(pk), coredomain.USDC(100))
		}
		car := fmt.Sprintf("RS-%s-AAA%017d", passoFundoMun, i)
		pid := coredomain.NewID()
		now := time.Now()
		if _, err := db.ExecContext(ctx, `
			INSERT INTO properties (id, user_id, car_hmac, micro_region_id, car_status, verified_at)
			VALUES ($1,$2,$3,$4,'approved',$5)`,
			pid, uid, auth.HMACIdentifier(car), regionID, now); err != nil {
			return err
		}
		prods = append(prods, prod{uid, wid, pid, pk})
	}

	labels := []string{"2023/24", "2024/25", "2025/26"}
	var cycleIDs []uuid.UUID
	base := time.Date(2023, 9, 1, 0, 0, 0, 0, time.UTC)
	for i, label := range labels {
		cid := coredomain.NewID()
		opens := base.AddDate(i, 0, 0)
		closes := opens.AddDate(0, 6, 0)
		_, err := db.ExecContext(ctx, `
			INSERT INTO cycles (id, culture_id, micro_region_id, label, opens_at, closes_at, status)
			VALUES ($1,$2,$3,$4,$5,$6,'aggregated')
			ON CONFLICT (culture_id, micro_region_id, label) DO NOTHING`,
			cid, cultureID, regionID, label, opens, closes)
		if err != nil {
			return err
		}
		_ = db.GetContext(ctx, &cid, `SELECT id FROM cycles WHERE culture_id=$1 AND micro_region_id=$2 AND label=$3`, cultureID, regionID, label)
		cycleIDs = append(cycleIDs, cid)
		for _, p := range prods {
			if p.walletID == uuid.Nil {
				continue
			}
			cidb := coredomain.NewID()
			_, err := db.ExecContext(ctx, `
				INSERT INTO contributions (id, cycle_id, wallet_id, property_id, level, attempt, commit_hash, commit_tx, commit_at, status)
				VALUES ($1,$2,$3,$4,'basic',1,'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','mock_demo', now(), 'accepted')
				ON CONFLICT DO NOTHING`, cidb, cid, p.walletID, p.propID)
			if err != nil {
				continue
			}
			_, _ = db.ExecContext(ctx, `
				INSERT INTO validated_metrics (id, contribution_id, metric, value) VALUES
				($1,$2,'area_ha',50), ($3,$2,'total_cost_ha',4200)`,
				coredomain.NewID(), cidb, coredomain.NewID())
		}
		// agrega
		_, _ = db.ExecContext(ctx, `
			INSERT INTO aggregates (id, cycle_id, level, metric, mean, median, p25, p75, n)
			VALUES ($1,$2,'basic','area_ha',50,50,40,60,14),
			       ($3,$2,'basic','total_cost_ha',4200,4200,3800,4600,14)
			ON CONFLICT DO NOTHING`, coredomain.NewID(), cid, coredomain.NewID())
		for _, p := range prods {
			if p.walletID == uuid.Nil {
				continue
			}
			_, _ = db.ExecContext(ctx, `
				INSERT INTO contribution_weights (id, cycle_id, wallet_id, weight)
				VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`,
				coredomain.NewID(), cid, p.walletID, 1.0/14.0)
		}
	}

	// instituição
	instEmail := "instituicao@agrobench.local"
	if _, err := users.GetByEmail(ctx, instEmail); err != nil {
		ihash, _ := crypto.HashPassword("demo12345")
		iu := iddomain.User{
			ID: coredomain.NewID(), Email: instEmail, Phone: "+5554999110000",
			CPFHMAC: auth.HMACIdentifier("CNPJ:12345678000199"), PasswordHash: ihash,
			Role: coredomain.RoleInstitution, MFAEnabled: false, Status: iddomain.UserActive,
		}
		_ = users.Create(ctx, iu)
		instID := coredomain.NewID()
		_, _ = db.ExecContext(ctx, `
			INSERT INTO institutions (id, user_id, name, cnpj_hmac, status, approved_at)
			VALUES ($1,$2,'Cooperativa Demo',$3,'approved', now())`,
			instID, iu.ID, auth.HMACIdentifier("12345678000199"))
		subID := coredomain.NewID()
		_, _ = db.ExecContext(ctx, `
			INSERT INTO subscriptions (id, institution_id, plan, regions, period_start, period_end, status)
			VALUES ($1,$2,'regional', ARRAY[$3]::uuid[], now(), now() + interval '1 year', 'active')`,
			subID, instID, regionID)
		for _, cid := range cycleIDs {
			_, _ = db.ExecContext(ctx, `
				INSERT INTO report_access (id, institution_id, subscription_id, cycle_id)
				VALUES ($1,$2,$3,$4)`, coredomain.NewID(), instID, subID, cid)
		}
	}

	if adapters != nil && adapters.ChainMock != nil {
		_ = adapters.ChainMock.Mint(ctx, adapters.Chain.Pool(), coredomain.USDC(5000))
	}
	periodID := coredomain.NewID()
	_, _ = db.ExecContext(ctx, `
		INSERT INTO pool_periods (id, month, gross, infra_cost, maintainer_fee, net, status)
		VALUES ($1,'2025-08',5000,280,750,3970,'distributed')
		ON CONFLICT (month) DO NOTHING`, periodID)
	return nil
}

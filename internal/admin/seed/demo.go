package seed

import (
	"context"
	"fmt"
	"log/slog"

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

// Wallet Solana Devnet do produtor da demo (Phantom). GET /wallet lê o saldo real.
const demoProducerPubkey = "FXsin7UZTGrix1cEe1QpMDFz3a8cDHzVK7h2oisjpzf3"

const demoProducerEmail = "produtor@agrobench.local"
const demoProducerPhone = "+5554999000001"
const demoPassword = "demo12345"

const demoInstitutionEmail = "instituicao@agrobench.local"
const demoInstitutionName = "Cotrijal — Cooperativa Agropecuária e Industrial"

type producerSeed struct {
	userID, walletID, propID uuid.UUID
	pubkey                   string
}

// Demo cria o conjunto da banca: 3 logins oficiais intactos, agricultores extras
// em várias microrregiões do RS (wallets off-chain só para média), ciclos
// agregados + uma janela 2026/27 aberta em Passo Fundo × soja, Cotrijal e pool.
func Demo(ctx context.Context, db *sqlx.DB, adapters *registry.Adapters) error {
	if _, err := Base(ctx, db, adapters); err != nil {
		return err
	}
	if err := dropLegacyDemoProducers(ctx, db); err != nil {
		return fmt.Errorf("limpando produtores fake: %w", err)
	}
	if err := dropExtraSeedFarmers(ctx, db); err != nil {
		return fmt.Errorf("limpando agricultores extras: %w", err)
	}

	hash, err := crypto.HashPassword(demoPassword)
	if err != nil {
		return err
	}

	main, err := ensureMainProducer(ctx, db, adapters, hash)
	if err != nil {
		return err
	}

	extras, err := createExtraFarmers(ctx, db, hash)
	if err != nil {
		return err
	}

	cycles, err := ensureHarvestCycles(ctx, db)
	if err != nil {
		return err
	}
	if err := seedHarvestContributions(ctx, db, cycles, main, extras); err != nil {
		return err
	}
	if err := recomputeAggregates(ctx, db, cycles); err != nil {
		return err
	}
	if err := ensureInstitution(ctx, db, hash, cycles); err != nil {
		return err
	}
	if err := ensurePool(ctx, db, adapters); err != nil {
		return err
	}
	return nil
}

func ensureMainProducer(ctx context.Context, db *sqlx.DB, adapters *registry.Adapters, hash string) (producerSeed, error) {
	var regionID uuid.UUID
	if err := db.GetContext(ctx, &regionID, `SELECT id FROM micro_regions WHERE ibge_code = $1`, passoFundoIBGE); err != nil {
		return producerSeed{}, fmt.Errorf("passo fundo: %w", err)
	}

	users := idrepo.NewUserRepo(db)
	var p producerSeed
	if existing, err := users.GetByEmail(ctx, demoProducerEmail); err == nil {
		p.userID = existing.ID
		_ = db.GetContext(ctx, &p.walletID, `SELECT id FROM wallets WHERE user_id = $1`, existing.ID)
		_ = db.GetContext(ctx, &p.pubkey, `SELECT pubkey FROM wallets WHERE user_id = $1`, existing.ID)
		_ = db.GetContext(ctx, &p.propID, `SELECT id FROM properties WHERE user_id = $1`, existing.ID)
		if p.walletID != uuid.Nil && p.pubkey != demoProducerPubkey {
			if _, err := db.ExecContext(ctx, `UPDATE wallets SET pubkey = $2, updated_at = now() WHERE id = $1`, p.walletID, demoProducerPubkey); err != nil {
				return producerSeed{}, fmt.Errorf("atualizando pubkey: %w", err)
			}
			p.pubkey = demoProducerPubkey
		}
	} else {
		uid := coredomain.NewID()
		u := iddomain.User{
			ID: uid, Email: demoProducerEmail, Phone: demoProducerPhone,
			CPFHMAC:      auth.HMACIdentifier("10000000001"),
			PasswordHash: hash, Role: coredomain.RoleProducer, MFAEnabled: true, Status: iddomain.UserActive,
		}
		if err := users.Create(ctx, u); err != nil {
			return producerSeed{}, err
		}
		p.userID = uid
		p.walletID = coredomain.NewID()
		p.pubkey = demoProducerPubkey
		// Blob dummy de propósito: o front não consegue decryptWalletBlob/"sign" neste login.
		// Lock on-chain exige secretbox nacl (64 bytes) no localStorage do aparelho.
		// Não reescrever a seed com chave real; conta criada no browser (ensureWallet) assina.
		if _, err := db.ExecContext(ctx, `
			INSERT INTO wallets (id, user_id, pubkey, encrypted_blob, blob_version)
			VALUES ($1,$2,$3,$4,1)`, p.walletID, uid, p.pubkey, []byte("demo")); err != nil {
			return producerSeed{}, err
		}
		car := fmt.Sprintf("RS-%s-AAA%017d", passoFundoMun, 1)
		p.propID = coredomain.NewID()
		if _, err := db.ExecContext(ctx, `
			INSERT INTO properties (id, user_id, car_hmac, micro_region_id, car_status, verified_at)
			VALUES ($1,$2,$3,$4,'approved',$5)`,
			p.propID, uid, auth.HMACIdentifier(car), regionID, nowUTC()); err != nil {
			return producerSeed{}, err
		}
	}
	if adapters != nil && p.pubkey != "" {
		if adapters.ChainMock != nil {
			_ = adapters.ChainMock.Mint(ctx, port.Account(p.pubkey), coredomain.USDC(100))
		} else if adapters.Chain != nil {
			fundDemoProducer(ctx, adapters.Chain, p.pubkey)
		}
	}
	return p, nil
}

func fundDemoProducer(ctx context.Context, chain port.ChainClient, pubkey string) {
	need := coredomain.USDC(20)
	bal, err := chain.Balance(ctx, port.Account(pubkey))
	if err != nil {
		slog.Warn("seed: não leu saldo USDC do produtor", "err", err)
		bal = 0
	}
	if bal >= need {
		return
	}
	delta := need - bal
	_, err = chain.TransferUSDC(ctx, port.TransferRequest{
		From: chain.Treasury(), To: port.Account(pubkey), Amount: delta, Memo: "seed:producer-stake",
	})
	if err != nil {
		slog.Warn("seed: não financiou USDC do produtor (lock de stake vai falhar)", "err", err)
	}
}

func ensureInstitution(ctx context.Context, db *sqlx.DB, hash string, cycles map[cycleKey]uuid.UUID) error {
	users := idrepo.NewUserRepo(db)
	existing, err := users.GetByEmail(ctx, demoInstitutionEmail)
	var instID uuid.UUID
	if err != nil {
		iu := iddomain.User{
			ID: coredomain.NewID(), Email: demoInstitutionEmail, Phone: "+5554999110000",
			CPFHMAC: auth.HMACIdentifier("CNPJ:12345678000199"), PasswordHash: hash,
			Role: coredomain.RoleInstitution, MFAEnabled: false, Status: iddomain.UserActive,
		}
		if err := users.Create(ctx, iu); err != nil {
			return fmt.Errorf("instituição: %w", err)
		}
		instID = coredomain.NewID()
		if _, err := db.ExecContext(ctx, `
			INSERT INTO institutions (id, user_id, name, cnpj_hmac, status, approved_at)
			VALUES ($1,$2,$3,$4,'approved', now())`,
			instID, iu.ID, demoInstitutionName, auth.HMACIdentifier("12345678000199")); err != nil {
			return err
		}
	} else {
		if err := db.GetContext(ctx, &instID, `SELECT id FROM institutions WHERE user_id = $1`, existing.ID); err != nil {
			return fmt.Errorf("instituição existente: %w", err)
		}
		if _, err := db.ExecContext(ctx, `UPDATE institutions SET name = $2 WHERE id = $1`, instID, demoInstitutionName); err != nil {
			return err
		}
	}

	var subID uuid.UUID
	err = db.GetContext(ctx, &subID, `
		SELECT id FROM subscriptions WHERE institution_id = $1 AND status = 'active'
		ORDER BY period_end DESC LIMIT 1`, instID)
	if err != nil {
		subID = coredomain.NewID()
		if _, err := db.ExecContext(ctx, `
			INSERT INTO subscriptions (id, institution_id, plan, regions, period_start, period_end, status)
			VALUES ($1,$2,'regional',
			        (SELECT array_agg(id) FROM micro_regions
			          WHERE ibge_code IN ('43010','43009','43012','43008','43011')),
			        now(), now() + interval '1 year', 'active')`,
			subID, instID); err != nil {
			return fmt.Errorf("assinatura Cotrijal: %w", err)
		}
	} else {
		if _, err := db.ExecContext(ctx, `
			UPDATE subscriptions SET
			  regions = (SELECT array_agg(id) FROM micro_regions
			              WHERE ibge_code IN ('43010','43009','43012','43008','43011')),
			  plan = 'regional',
			  period_end = GREATEST(period_end, now() + interval '1 year')
			WHERE id = $1`, subID); err != nil {
			return fmt.Errorf("atualizando assinatura: %w", err)
		}
	}
	_, _ = db.ExecContext(ctx, `
		UPDATE subscriptions SET status = 'cancelled'
		 WHERE institution_id = $1 AND id <> $2 AND status = 'active'`, instID, subID)

	for _, cid := range cycles {
		_, _ = db.ExecContext(ctx, `
			INSERT INTO report_access (id, institution_id, subscription_id, cycle_id)
			SELECT $1,$2,$3,$4
			 WHERE NOT EXISTS (
			   SELECT 1 FROM report_access WHERE institution_id=$2 AND cycle_id=$4
			 )`, coredomain.NewID(), instID, subID, cid)
	}
	return nil
}

func ensurePool(ctx context.Context, db *sqlx.DB, adapters *registry.Adapters) error {
	if adapters != nil && adapters.ChainMock != nil {
		_ = adapters.ChainMock.Mint(ctx, adapters.Chain.Pool(), coredomain.USDC(5000))
	}
	_, _ = db.ExecContext(ctx, `
		INSERT INTO pool_periods (id, month, gross, infra_cost, maintainer_fee, net, status)
		VALUES ($1,'2025-08',5000,20,750,4230,'distributed')
		ON CONFLICT (month) DO NOTHING`, coredomain.NewID())
	_, _ = db.ExecContext(ctx, `
		INSERT INTO pool_periods (id, month, gross, infra_cost, maintainer_fee, net, status)
		VALUES ($1,'2026-08',4500,40,675,3785,'distributed')
		ON CONFLICT (month) DO NOTHING`, coredomain.NewID())
	_, _ = db.ExecContext(ctx, `
		INSERT INTO pool_periods (id, month, gross, infra_cost, maintainer_fee, net, status)
		VALUES ($1,'2026-09',800,8,120,672,'open')
		ON CONFLICT (month) DO NOTHING`, coredomain.NewID())
	return nil
}

// dropLegacyDemoProducers apaga produtor01@…–produtor14@… e as linhas on-chain fake.
func dropLegacyDemoProducers(ctx context.Context, db *sqlx.DB) error {
	return deleteUsersMatching(ctx, db, `email ~ '^produtor[0-9]{2}@agrobench\.local$'`)
}

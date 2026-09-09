package seed

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"fmt"

	ag "github.com/gagliardetto/solana-go"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/core/auth"
	"github.com/AgroBench/backend/internal/core/crypto"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	iddomain "github.com/AgroBench/backend/internal/identity/domain"
	idrepo "github.com/AgroBench/backend/internal/identity/repository"
)

// Agricultores extras da demo (médias / rankings). Não são logins oficiais.
// Email: seed.farmer.NN@agrobench.local — senha dummy (mesmo hash da demo).
// Wallet: pubkey ed25519 válida em formato Solana, NÃO funded na Devnet.
const extraFarmerEmail = `email ~ '^seed\.farmer\.[0-9]+@agrobench\.local$'`

type extraFarm struct {
	seq        int
	regionIBGE string
	munIBGE    string
	culture    string
	areaHa     float64
	costHa     float64
	yieldHa    float64
}

type farmGroup struct {
	regionIBGE, munIBGE, culture string
	count                        int
}

// Municípios IBGE (7 dígitos) das microrregiões usadas na seed rica.
const (
	carazinhoMun    = "4304705"
	naoMeToqueMun   = "4312658"
	ijuiMun         = "4310207"
	cruzAltaMun     = "4306106"
	erechimMun      = "4307005"
	santaRosaMun    = "4317202"
	vacariaMun      = "4322509"
	santoAngeloMun  = "4317509"
	carazinhoIBGE   = "43009"
	naoMeToqueIBGE  = "43012"
	ijuiIBGE        = "43008"
	cruzAltaIBGE    = "43011"
	erechimIBGE     = "43004"
	santaRosaIBGE   = "43001"
	vacariaIBGE     = "43015"
	santoAngeloIBGE = "43007"
)

// 5 microrregiões do plano regional da Cotrijal (máx. plans.regional.max_regions):
// Passo Fundo, Carazinho, Não-Me-Toque, Ijuí, Cruz Alta.
var extraFarmGroups = []farmGroup{
	{passoFundoIBGE, passoFundoMun, "soybean", 7},
	{passoFundoIBGE, passoFundoMun, "corn", 3},
	{passoFundoIBGE, passoFundoMun, "wheat", 2},
	{carazinhoIBGE, carazinhoMun, "soybean", 4},
	{naoMeToqueIBGE, naoMeToqueMun, "soybean", 4},
	{ijuiIBGE, ijuiMun, "corn", 3},
	{cruzAltaIBGE, cruzAltaMun, "soybean", 3},
	{erechimIBGE, erechimMun, "wheat", 3},
	{santaRosaIBGE, santaRosaMun, "soybean", 3},
	{vacariaIBGE, vacariaMun, "wheat", 3},
	{santoAngeloIBGE, santoAngeloMun, "soybean", 3},
}

func extraFarms() []extraFarm {
	out := make([]extraFarm, 0, 40)
	seq := 0
	for _, g := range extraFarmGroups {
		for i := 0; i < g.count; i++ {
			seq++
			area, cost, yield := plausibleMetrics(g.culture, seq)
			out = append(out, extraFarm{
				seq: seq, regionIBGE: g.regionIBGE, munIBGE: g.munIBGE,
				culture: g.culture, areaHa: area, costHa: cost, yieldHa: yield,
			})
		}
	}
	return out
}

func extraFarmerEmailOf(seq int) string {
	return fmt.Sprintf("seed.farmer.%02d@agrobench.local", seq)
}

// plausibleMetrics devolve área, custo/ha e sc/ha dentro das faixas CONAB mock.
func plausibleMetrics(culture string, seq int) (area, cost, yield float64) {
	switch culture {
	case "corn":
		area = 60 + float64((seq*41)%130)
		cost = 3900 + float64((seq*211)%1200)
		yield = 95 + float64((seq*13)%40)
	case "wheat":
		area = 35 + float64((seq*29)%80)
		cost = 2700 + float64((seq*157)%1300)
		yield = 36 + float64((seq*11)%18)
	default: // soybean
		area = 55 + float64((seq*37)%140)
		cost = 4450 + float64((seq*173)%1100)
		yield = 54 + float64((seq*7)%14)
	}
	return
}

// offchainPubkey gera uma pubkey Solana (base58) determinística e válida,
// sem keypair na Devnet — só serve para FK de wallet/contribuição.
func offchainPubkey(label string) string {
	seed := sha256.Sum256([]byte("agrobench.seed.offchain.v1:" + label))
	priv := ed25519.NewKeyFromSeed(seed[:])
	return ag.PublicKeyFromBytes(priv.Public().(ed25519.PublicKey)).String()
}

type seededFarmer struct {
	seq        int
	culture    string
	regionIBGE string
	userID     uuid.UUID
	walletID   uuid.UUID
	propID     uuid.UUID
	areaHa     float64
	costHa     float64
	yieldHa    float64
}

func createExtraFarmers(ctx context.Context, db *sqlx.DB, passwordHash string) ([]seededFarmer, error) {
	users := idrepo.NewUserRepo(db)
	farms := extraFarms()
	out := make([]seededFarmer, 0, len(farms))
	now := nowUTC()

	regionIDs := map[string]uuid.UUID{}
	for _, f := range farms {
		if _, ok := regionIDs[f.regionIBGE]; ok {
			continue
		}
		var id uuid.UUID
		if err := db.GetContext(ctx, &id, `SELECT id FROM micro_regions WHERE ibge_code = $1`, f.regionIBGE); err != nil {
			return nil, fmt.Errorf("microrregião %s: %w", f.regionIBGE, err)
		}
		regionIDs[f.regionIBGE] = id
	}

	for _, f := range farms {
		email := extraFarmerEmailOf(f.seq)
		car := fmt.Sprintf("RS-%s-AAA%017d", f.munIBGE, 100+f.seq)
		if _, err := db.ExecContext(ctx, `
			INSERT INTO mock_sicar_cars (car_number, active, uf, ibge_code)
			VALUES ($1, true, 'RS', $2)
			ON CONFLICT (car_number) DO NOTHING`,
			crypto.NormalizeIdentifier(car), f.munIBGE); err != nil {
			return nil, fmt.Errorf("mock car %s: %w", car, err)
		}

		uid := coredomain.NewID()
		u := iddomain.User{
			ID: uid, Email: email, Phone: fmt.Sprintf("+5554988%05d", f.seq),
			CPFHMAC:      auth.HMACIdentifier(fmt.Sprintf("%011d", 40000000000+f.seq)),
			PasswordHash: passwordHash, Role: coredomain.RoleProducer,
			MFAEnabled: false, Status: iddomain.UserActive,
		}
		if err := users.Create(ctx, u); err != nil {
			return nil, fmt.Errorf("agricultor extra %s: %w", email, err)
		}
		wid := coredomain.NewID()
		if _, err := db.ExecContext(ctx, `
			INSERT INTO wallets (id, user_id, pubkey, encrypted_blob, blob_version)
			VALUES ($1,$2,$3,$4,1)`, wid, uid, offchainPubkey(email), []byte("seed-offchain")); err != nil {
			return nil, fmt.Errorf("wallet extra %s: %w", email, err)
		}
		pid := coredomain.NewID()
		if _, err := db.ExecContext(ctx, `
			INSERT INTO properties (id, user_id, car_hmac, micro_region_id, car_status, verified_at)
			VALUES ($1,$2,$3,$4,'approved',$5)`,
			pid, uid, auth.HMACIdentifier(car), regionIDs[f.regionIBGE], now); err != nil {
			return nil, fmt.Errorf("propriedade extra %s: %w", email, err)
		}
		out = append(out, seededFarmer{
			seq: f.seq, culture: f.culture, regionIBGE: f.regionIBGE,
			userID: uid, walletID: wid, propID: pid,
			areaHa: f.areaHa, costHa: f.costHa, yieldHa: f.yieldHa,
		})
	}
	return out, nil
}

func dropExtraSeedFarmers(ctx context.Context, db *sqlx.DB) error {
	return deleteUsersMatching(ctx, db, extraFarmerEmail)
}

func deleteUsersMatching(ctx context.Context, db *sqlx.DB, where string) error {
	stmts := []string{
		`DELETE FROM payouts WHERE wallet_id IN (SELECT id FROM wallets WHERE user_id IN (SELECT id FROM users WHERE ` + where + `))`,
		`DELETE FROM contribution_weights WHERE wallet_id IN (SELECT id FROM wallets WHERE user_id IN (SELECT id FROM users WHERE ` + where + `))`,
		`DELETE FROM attestations WHERE contribution_id IN (SELECT id FROM contributions WHERE wallet_id IN (SELECT id FROM wallets WHERE user_id IN (SELECT id FROM users WHERE ` + where + `)))`,
		`DELETE FROM validated_metrics WHERE contribution_id IN (SELECT id FROM contributions WHERE wallet_id IN (SELECT id FROM wallets WHERE user_id IN (SELECT id FROM users WHERE ` + where + `)))`,
		`DELETE FROM validation_verdicts WHERE contribution_id IN (SELECT id FROM contributions WHERE wallet_id IN (SELECT id FROM wallets WHERE user_id IN (SELECT id FROM users WHERE ` + where + `)))`,
		`DELETE FROM stakes WHERE contribution_id IN (SELECT id FROM contributions WHERE wallet_id IN (SELECT id FROM wallets WHERE user_id IN (SELECT id FROM users WHERE ` + where + `)))`,
		`DELETE FROM contributions WHERE wallet_id IN (SELECT id FROM wallets WHERE user_id IN (SELECT id FROM users WHERE ` + where + `))`,
		`DELETE FROM wallets WHERE user_id IN (SELECT id FROM users WHERE ` + where + `)`,
		`DELETE FROM properties WHERE user_id IN (SELECT id FROM users WHERE ` + where + `)`,
		`DELETE FROM users WHERE ` + where,
	}
	for _, q := range stmts {
		if _, err := db.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

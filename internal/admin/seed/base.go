package seed

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/core/auth"
	"github.com/AgroBench/backend/internal/core/crypto"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	iddomain "github.com/AgroBench/backend/internal/identity/domain"
	idrepo "github.com/AgroBench/backend/internal/identity/repository"
	"github.com/AgroBench/backend/pkg/adapter/registry"
)

// Base insere microrregiões do RS, culturas, CARs fictícios, faixas CONAB e o admin.
func Base(ctx context.Context, db *sqlx.DB, adapters *registry.Adapters) (int, error) {
	n := 0
	for _, r := range rsMicroRegions {
		_, err := db.ExecContext(ctx, `
			INSERT INTO micro_regions (id, ibge_code, name, uf)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (ibge_code) DO NOTHING`,
			coredomain.NewID(), r.code, r.name, "RS")
		if err != nil {
			return n, fmt.Errorf("micro_region %s: %w", r.code, err)
		}
		n++
	}
	for _, c := range cultures {
		_, err := db.ExecContext(ctx, `
			INSERT INTO cultures (id, code, name) VALUES ($1, $2, $3)
			ON CONFLICT (code) DO NOTHING`,
			coredomain.NewID(), c.code, c.name)
		if err != nil {
			return n, fmt.Errorf("culture %s: %w", c.code, err)
		}
		n++
	}
	if err := seedMockCars(ctx, db); err != nil {
		return n, err
	}
	if err := seedMockRanges(ctx, db); err != nil {
		return n, err
	}
	if err := seedAdmin(ctx, db); err != nil {
		return n, err
	}
	if adapters != nil && adapters.ChainMock != nil {
		_ = adapters.ChainMock.Mint(ctx, adapters.Chain.Treasury(), coredomain.USDC(1_000_000))
	}
	return n, nil
}

func seedAdmin(ctx context.Context, db *sqlx.DB) error {
	email := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_EMAIL")))
	if email == "" {
		email = "admin@agrobench.local"
	}
	pass := os.Getenv("ADMIN_PASSWORD")
	if pass == "" {
		pass = "admin123"
	}
	hash, err := crypto.HashPassword(pass)
	if err != nil {
		return err
	}
	users := idrepo.NewUserRepo(db)
	if _, err := users.GetByEmail(ctx, email); err == nil {
		return nil
	}
	return users.Create(ctx, iddomain.User{
		ID:           coredomain.NewID(),
		Email:        email,
		Phone:        "+5554999990000",
		CPFHMAC:      auth.HMACIdentifier("00000000000"),
		PasswordHash: hash,
		Role:         coredomain.RoleAdmin,
		MFAEnabled:   false,
		Status:       iddomain.UserActive,
	})
}

type named struct{ code, name string }

var cultures = []named{
	{"soybean", "Soja"},
	{"corn", "Milho"},
	{"wheat", "Trigo"},
}

// 35 microrregiões IBGE do RS. Passo Fundo = 43010 (README §13).
var rsMicroRegions = []named{
	{"43001", "Santa Rosa"},
	{"43002", "Três Passos"},
	{"43003", "Frederico Westphalen"},
	{"43004", "Erechim"},
	{"43005", "Sananduva"},
	{"43006", "Cerro Largo"},
	{"43007", "Santo Ângelo"},
	{"43008", "Ijuí"},
	{"43009", "Carazinho"},
	{"43010", "Passo Fundo"},
	{"43011", "Cruz Alta"},
	{"43012", "Não-Me-Toque"},
	{"43013", "Soledade"},
	{"43014", "Guaporé"},
	{"43015", "Vacaria"},
	{"43016", "Caxias do Sul"},
	{"43017", "Santiago"},
	{"43018", "Santa Maria"},
	{"43019", "Restinga Seca"},
	{"43020", "Santa Cruz do Sul"},
	{"43021", "Lajeado-Estrela"},
	{"43022", "Cachoeira do Sul"},
	{"43023", "Montenegro"},
	{"43024", "Gramado-Canela"},
	{"43025", "São Jerônimo"},
	{"43026", "Porto Alegre"},
	{"43027", "Osório"},
	{"43028", "Camaquã"},
	{"43029", "Campanha Ocidental"},
	{"43030", "Campanha Central"},
	{"43031", "Campanha Meridional"},
	{"43032", "Serras de Sudeste"},
	{"43033", "Pelotas"},
	{"43034", "Jaguarão"},
	{"43035", "Litoral Lagunar"},
}

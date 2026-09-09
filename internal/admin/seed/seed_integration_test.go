package seed_test

import (
	"context"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/AgroBench/backend/internal/admin/seed"
	"github.com/AgroBench/backend/internal/testutil"
	"github.com/AgroBench/backend/pkg/adapter/registry"
)

func TestSeedBaseIntegration(t *testing.T) {
	db := testutil.Postgres(t)
	viper.Set("auth.pepper", "test-pepper-32-bytes-minimum-ok")
	viper.Set("adapters.chain", "mock")
	viper.Set("adapters.enclave", "mock")
	viper.Set("adapters.sicar", "mock")
	viper.Set("adapters.reference_data", "mock")
	viper.Set("adapters.sms", "mock")
	viper.Set("adapters.payment", "mock")
	adapters, err := registry.Build(db)
	require.NoError(t, err)
	n, err := seed.Base(context.Background(), db, adapters)
	require.NoError(t, err)
	require.Greater(t, n, 30)
	var cultures int
	require.NoError(t, db.Get(&cultures, `SELECT COUNT(*) FROM cultures`))
	require.Equal(t, 3, cultures)
}

func TestSeedDemoIntegration(t *testing.T) {
	db := testutil.Postgres(t)
	viper.Set("auth.pepper", "test-pepper-32-bytes-minimum-ok")
	viper.Set("adapters.chain", "mock")
	viper.Set("adapters.enclave", "mock")
	viper.Set("adapters.sicar", "mock")
	viper.Set("adapters.reference_data", "mock")
	viper.Set("adapters.sms", "mock")
	viper.Set("adapters.payment", "mock")
	adapters, err := registry.Build(db)
	require.NoError(t, err)
	require.NoError(t, seed.Demo(context.Background(), db, adapters))

	var email string
	require.NoError(t, db.Get(&email, `SELECT email FROM users WHERE email = $1`, "produtor@agrobench.local"))
	require.NoError(t, db.Get(&email, `SELECT email FROM users WHERE email = $1`, "instituicao@agrobench.local"))
	require.NoError(t, db.Get(&email, `SELECT email FROM users WHERE email = $1`, "admin@agrobench.local"))

	var pubkey string
	require.NoError(t, db.Get(&pubkey, `
		SELECT w.pubkey FROM wallets w JOIN users u ON u.id = w.user_id WHERE u.email = $1`,
		"produtor@agrobench.local"))
	require.Equal(t, "FXsin7UZTGrix1cEe1QpMDFz3a8cDHzVK7h2oisjpzf3", pubkey)

	var producers, extras, regions, openCycles, maxN int
	require.NoError(t, db.Get(&producers, `SELECT COUNT(*) FROM users WHERE role = 'producer'`))
	require.GreaterOrEqual(t, producers, 30)
	require.NoError(t, db.Get(&extras, `SELECT COUNT(*) FROM users WHERE email ~ '^seed\.farmer\.'`))
	require.GreaterOrEqual(t, extras, 25)
	require.NoError(t, db.Get(&regions, `SELECT COUNT(DISTINCT micro_region_id) FROM properties`))
	require.GreaterOrEqual(t, regions, 8)
	require.NoError(t, db.Get(&openCycles, `SELECT COUNT(*) FROM cycles WHERE status = 'open'`))
	require.GreaterOrEqual(t, openCycles, 1)
	require.NoError(t, db.Get(&maxN, `SELECT COALESCE(MAX(n),0) FROM aggregates WHERE metric = 'total_cost_ha'`))
	require.GreaterOrEqual(t, maxN, 5)

	var instName string
	require.NoError(t, db.Get(&instName, `SELECT name FROM institutions LIMIT 1`))
	require.Contains(t, instName, "Cotrijal")

	require.NoError(t, seed.Demo(context.Background(), db, adapters))
	require.NoError(t, db.Get(&producers, `SELECT COUNT(*) FROM users WHERE role = 'producer'`))
	require.GreaterOrEqual(t, producers, 30)
}

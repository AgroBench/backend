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

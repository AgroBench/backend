// Package testutil sobe um Postgres real via testcontainers para testes de integração.
// Use `go test -short` para pular. Todo teste de integração se chama Test...Integration.
package testutil

import (
	"context"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/AgroBench/backend/migrations"
)

// Postgres sobe um container postgres:17-alpine, aplica as migrations e devolve a conexão.
// O container é destruído no Cleanup do teste.
func Postgres(t *testing.T) *sqlx.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("integração: pulado com -short")
	}

	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("agrobench_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("subindo postgres: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = ctr.Terminate(ctx)
	})

	url, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	if err := migrations.Up(url); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	db, err := sqlx.Connect("pgx", url)
	if err != nil {
		t.Fatalf("conectando: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

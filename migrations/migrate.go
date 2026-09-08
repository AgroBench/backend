// Package migrations embute os arquivos SQL no binário e expõe Up/Down/Version via golang-migrate.
// Convenção de nomes: NNNNNN_descricao.up.sql / .down.sql. Crie novos com `make migrate-create name=x`.
package migrations

import (
	"embed"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed *.sql
var files embed.FS

func newMigrator(databaseURL string) (*migrate.Migrate, error) {
	src, err := iofs.New(files, ".")
	if err != nil {
		return nil, fmt.Errorf("migrations embed: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, toPgxURL(databaseURL))
	if err != nil {
		return nil, fmt.Errorf("migrate init: %w", err)
	}
	return m, nil
}

// toPgxURL troca o scheme postgres:// ou postgresql:// por pgx5://, exigido pelo driver pgx/v5 do golang-migrate.
func toPgxURL(databaseURL string) string {
	for _, scheme := range []string{"postgresql://", "postgres://"} {
		if strings.HasPrefix(databaseURL, scheme) {
			return "pgx5://" + strings.TrimPrefix(databaseURL, scheme)
		}
	}
	return databaseURL
}

func Up(databaseURL string) error {
	m, err := newMigrator(databaseURL)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// Down reverte UMA migration (a última aplicada).
func Down(databaseURL string) error {
	m, err := newMigrator(databaseURL)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Steps(-1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}

func Version(databaseURL string) (uint, bool, error) {
	m, err := newMigrator(databaseURL)
	if err != nil {
		return 0, false, err
	}
	defer m.Close()
	v, dirty, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, nil
	}
	return v, dirty, err
}

// Package database abre a conexão sqlx com o Postgres usando o driver pgx (stdlib).
package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/spf13/viper"
)

func Connect(ctx context.Context) (*sqlx.DB, error) {
	url := viper.GetString("database.url")
	if url == "" {
		return nil, fmt.Errorf("database.url não configurada")
	}

	db, err := sqlx.Open("pgx", url)
	if err != nil {
		return nil, fmt.Errorf("abrindo conexão: %w", err)
	}
	db.SetMaxOpenConns(viper.GetInt("database.max_open_conns"))
	db.SetMaxIdleConns(viper.GetInt("database.max_idle_conns"))
	db.SetConnMaxLifetime(30 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping no postgres: %w", err)
	}

	slog.Info("postgres conectado")
	return db, nil
}

// Package config carrega o config.json via viper e aplica override por variáveis de
// ambiente: a chave "database.url" pode ser sobrescrita por DATABASE_URL, e assim por diante.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const defaultConfigFile = "config/env/config.dev.json"

func Load(path string) error {
	if path == "" {
		path = os.Getenv("CONFIG_FILE")
	}
	if path == "" {
		path = defaultConfigFile
	}

	viper.SetConfigFile(path)
	viper.SetConfigType("json")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("lendo %s: %w", path, err)
	}

	// PORT é a convenção dos PaaS; mapeia pra server.http.port sem exigir SERVER_HTTP_PORT.
	if p := os.Getenv("PORT"); p != "" {
		viper.Set("server.http.port", p)
	}

	if tz := viper.GetString("app.timezone"); tz != "" {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			return fmt.Errorf("timezone inválida %q: %w", tz, err)
		}
		time.Local = loc
	}

	return nil
}

func SetupLogger() {
	var level slog.Level
	switch strings.ToLower(viper.GetString("log.level")) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if viper.GetString("log.format") == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(handler).With("app", viper.GetString("app.name")))
}

func IsDev() bool {
	return viper.GetString("app.env") == "dev"
}

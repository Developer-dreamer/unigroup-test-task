package config

import (
	"fmt"
	"github.com/ilyakaznacheev/cleanenv"
	"log/slog"
	"os"
	"unigroup-test-task/internal"
)

func LoadCfgFilesDir() string {
	str := os.Getenv("YAML_CFG_DIR")
	if str == "" {
		return "../../config/app/api.yaml"
	}
	return str
}

func Load[T APIConfig | NotifConfig](path string) (*T, error) {
	var cfg T

	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		if err := cleanenv.ReadEnv(&cfg); err != nil {
			return nil, fmt.Errorf("config error: %w", err)
		}
	}

	return &cfg, nil
}

func NewLogger(level slog.Level) *slog.Logger {
	baseHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	l := slog.New(internal.TraceHandler{Handler: baseHandler})
	slog.SetDefault(l)

	return l
}

package main

import (
	"log"
	"log/slog"
	"unigroup-test-task/internal/app"
	"unigroup-test-task/internal/config"
)

func main() {
	logger := config.NewLogger(slog.LevelDebug)

	configPath := config.LoadCfgFilesDir()
	logger.Info("Loading path", "path", configPath)

	cfg, err := config.Load[config.APIConfig](configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	logger.Info("Loading cfg", "redisURI", cfg.Redis.URI)

	server, producer := app.SetupHttpServer(cfg, logger)
	app.GracefulShutdown(server, producer, logger)
}

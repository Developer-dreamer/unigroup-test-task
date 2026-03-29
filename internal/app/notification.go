package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"unigroup-test-task/internal/config"
	"unigroup-test-task/internal/event"
)

func SetupListener(cfg *config.NotifConfig, l *slog.Logger) *event.Consumer {
	redisClient, err := config.ConnectToRedis(cfg.App.Environment, cfg.Redis.URI)
	if err != nil {
		l.Error("Failed to initiate redis. Server shutdown.", "error", err)
		os.Exit(1)
	}

	consumer, err := event.NewConsumer(l, redisClient, &cfg.Redis.SubStream, &cfg.App.Backoff)

	return consumer
}

func StartListener(logger *slog.Logger, cfg *config.NotifConfig, consumer *event.Consumer) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("Received termination signal. Stopping consumer...", "signal", sig)
		cancel()
	}()

	go addHealthCheck(logger, &cfg.App)

	if err := consumer.Consume(ctx); err != nil {
		if !errors.Is(err, context.Canceled) {
			logger.Error("Consumer failed", "error", err)
		}
	}

	logger.Info("System shutdown complete.")
}

func addHealthCheck(logger *slog.Logger, cfg *config.NotifAppConfig) {
	server := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Worker is running"))
		}),
	}

	logger.Info("Starting health check server", "port", cfg.Port)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("Failed to start health check server", "error", err)
		os.Exit(1)
	}
}

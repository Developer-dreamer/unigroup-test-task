package app

import (
	"context"
	"errors"
	"github.com/gorilla/mux"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unigroup-test-task/internal"
	"unigroup-test-task/internal/config"
	"unigroup-test-task/internal/event"
	"unigroup-test-task/internal/product"
)

func SetupHttpServer(cfg *config.APIConfig, l *slog.Logger) (*http.Server, *event.Relay) {
	redisClient, err := config.ConnectToRedis(cfg.App.Environment, cfg.Redis.URI)
	if err != nil {
		l.Error("Failed to initiate redis. Server shutdown.", "error", err)
		os.Exit(1)
	}
	postgresClient, err := config.ConnectToPostgres(cfg.Postgres)
	if err != nil {
		l.Error("Failed to initiate postgres. Server shutdown.", "error", err)
		os.Exit(1)
	}
	if err = config.RunMigrations(postgresClient, cfg.App.MigrationsDir); err != nil {
		l.Error("error while running migrations", "error", err)
		os.Exit(1)
	}

	transactor, err := internal.NewTransactor(l, postgresClient)
	if err != nil {
		l.Error("Failed to initiate transactor.", "error", err)
		os.Exit(1)
	}
	otbx, err := event.NewRepository(l, postgresClient)
	if err != nil {
		l.Error("Failed to initiate outbox.", "error", err)
		os.Exit(1)
	}

	producer, err := event.NewProducer(l, redisClient, &cfg.Redis.PubStream)
	if err != nil {
		l.Error("Failed to initiate producer.", "error", err)
		os.Exit(1)
	}
	relay, err := event.NewRelayService(l, transactor, otbx, producer, &cfg.App.Backoff)
	if err != nil {
		l.Error("Failed to initiate relay.", "error", err)
		os.Exit(1)
	}

	pr, err := product.NewRepository(l, postgresClient)
	if err != nil {
		l.Error("Failed to initiate prompt repository.", "error", err)
		os.Exit(1)
	}

	srvc, err := product.NewService(l, pr, otbx, transactor)
	if err != nil {
		l.Error("Failed to initiate product service.", "error", err)
		os.Exit(1)
	}

	ph, err := product.NewHandler(l, srvc)
	if err != nil {
		l.Error("Failed to initiate prompt handler.", "error", err)
		os.Exit(1)
	}

	r := registerRoutes(ph, l)

	l.Info("Starting server")

	return &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      r,
		IdleTimeout:  120 * time.Second,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
	}, relay
}

func registerRoutes(handler *product.Handler, logger internal.Logger) *mux.Router {
	r := mux.NewRouter()

	recoveryManager := internal.NewRecoveryManager(logger)
	r.Use(recoveryManager.Recovery)
	r.Use(internal.TracingMiddleware)

	r.HandleFunc("/products", handler.GetProducts).Methods(http.MethodGet)
	r.HandleFunc("/products", handler.PostProduct).Methods(http.MethodPost)
	r.HandleFunc("/products/{id}", handler.DeleteProduct).Methods(http.MethodDelete)

	r.HandleFunc("/health", healthCheck).Methods(http.MethodGet)

	return r
}

func healthCheck(rw http.ResponseWriter, _ *http.Request) {
	internal.WriteJSONResponse(rw, http.StatusOK, nil)
}

func GracefulShutdown(server *http.Server, relay *event.Relay, logger *slog.Logger) {
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("starting HTTP server", "addr", server.Addr, "idle_timeout", server.IdleTimeout, "read_timeout", server.ReadTimeout, "write_timeout", server.WriteTimeout)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("error occurred when starting server", "error", err)
			appCancel()
		}
	}()

	go func() {
		logger.Info("Starting relay")
		if err := relay.Start(appCtx); err != nil {
			if !errors.Is(err, context.Canceled) {
				logger.Error("error occurred in relay", "error", err)
			}
		}
	}()

	select {
	case sig := <-sigChan:
		logger.Info("received terminate signal", "sig", sig)
	case <-appCtx.Done():
		logger.Info("application context canceled (internal error)")
	}

	appCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("error during server shutdown", "error", err, "addr", server.Addr)
	}

	logger.Info("Graceful shutdown completed")
}

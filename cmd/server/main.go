// Command server runs the HTTP service and shuts down gracefully.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/httphandler"
	"github.com/ppa/itprj-0007-sso/internal/infra/caching"
	"github.com/ppa/itprj-0007-sso/internal/infra/config"
	"github.com/ppa/itprj-0007-sso/internal/infra/database"
	"github.com/ppa/itprj-0007-sso/internal/infra/logger"
)

var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "server:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	log := logger.New(cfg.Log.Level)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(ctx, cfg.Database.URL)
	if err != nil {
		return err
	}
	defer db.Close()

	redisClient, err := caching.New(cfg.Redis.URL)
	if err != nil {
		return err
	}
	defer func() { _ = redisClient.Close() }()

	handler := httphandler.NewRouter(httphandler.Deps{
		Config: cfg,
		DB:     db,
		Redis:  redisClient,
		Logger: log,
		Version: domain.VersionInfo{
			Version:   version,
			Commit:    commit,
			BuildDate: buildDate,
			GoVersion: runtime.Version(),
		},
	})

	server := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("server starting", "addr", cfg.HTTP.Addr, "env", cfg.AppEnv, "version", version)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("listen: %w", err)
	case <-ctx.Done():
	}

	log.Info("server stopping")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}

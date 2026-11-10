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
	"github.com/ppa/itprj-0007-sso/internal/pkg"
	"github.com/ppa/itprj-0007-sso/internal/pkg/password"
	"github.com/ppa/itprj-0007-sso/internal/repository"
	"github.com/ppa/itprj-0007-sso/internal/workerhandler"
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

	ring, err := pkg.NewKeyring(cfg.MasterKeys.Keys, cfg.MasterKeys.KeyID)
	if err != nil {
		return fmt.Errorf("build master keyring: %w", err)
	}

	users := repository.NewUserRepository(db.Pool)
	credentials := repository.NewCredentialRepository(db.Pool)
	sessions := repository.NewSessionRepository(db.Pool)
	roles := repository.NewRoleRepository(db.Pool)
	emailTokens := repository.NewEmailTokenRepository(db.Pool)
	audit := repository.NewAuditRepository(db.Pool)
	outbox := repository.NewOutboxRepository(db.Pool)
	realms := repository.NewRealmRepository(db.Pool)
	clients := repository.NewClientRepository(db.Pool)
	signingKeys := repository.NewSigningKeyRepository(db.Pool, ring)
	authRequests := repository.NewAuthRequestRepository(redisClient)
	authCodes := repository.NewAuthCodeRepository(redisClient)
	refreshTokens := repository.NewRefreshTokenRepository(db.Pool)
	tokenState := repository.NewTokenStateRepository(redisClient)
	csrfStore := repository.NewCSRFStore(redisClient)
	rateLimit := repository.NewRateLimitRepository(redisClient)
	mfa := repository.NewMFARepository(db.Pool)
	mfaChallenges := repository.NewMFAChallengeRepository(redisClient)
	adminUsers := repository.NewAdminUserRepository(db.Pool)
	idempotency := repository.NewIdempotencyRepository(db.Pool)
	adminClients := repository.NewAdminClientRepository(db.Pool)
	adminRoles := repository.NewAdminRoleRepository(db.Pool)
	groups := repository.NewGroupRepository(db.Pool)
	adminSessions := repository.NewAdminSessionRepository(db.Pool)
	adminAudit := repository.NewAdminAuditRepository(db.Pool)
	realmSettings := repository.NewRealmSettingsRepository(db.Pool)

	mailer := repository.NewSMTPSender(repository.SMTPSenderConfig{
		Host:     cfg.SMTP.Host,
		Port:     cfg.SMTP.Port,
		From:     cfg.SMTP.From,
		Username: cfg.SMTP.Username,
		Password: cfg.SMTP.Password,
		TLS:      cfg.SMTP.TLS,
	})
	worker := workerhandler.NewOutboxWorker(outbox, mailer, signingKeys,
		repository.NewLogoutSender(), log)
	worker.UseRealmMail(repository.NewSMTPSenderFactory(mailer, ring), realmSettings)
	go worker.Run(ctx)

	retention := workerhandler.NewRetentionWorker(audit, repository.NewRetentionRepository(db.Pool), log)
	go retention.Run(ctx)

	handler := httphandler.NewRouter(httphandler.Deps{
		Config:        cfg,
		DB:            db,
		Redis:         redisClient,
		Logger:        log,
		Realms:        realms,
		Users:         users,
		Credentials:   credentials,
		Sessions:      sessions,
		Roles:         roles,
		EmailTokens:   emailTokens,
		Audit:         audit,
		CSRFStore:     csrfStore,
		RateLimit:     rateLimit,
		Passwords:     password.NewGuard(cfg.Argon2.Concurrency),
		MFA:           mfa,
		MFAChallenges: mfaChallenges,
		AdminUsers:    adminUsers,
		Idempotency:   idempotency,
		Keyring:       ring,
		Clients:       clients,
		SigningKeys:   signingKeys,
		Tokens:        signingKeys,
		AuthRequests:  authRequests,
		AuthCodes:     authCodes,
		RefreshTokens: refreshTokens,
		TokenState:    tokenState,
		Outbox:        outbox,
		AdminClients:  adminClients,
		AdminRoles:    adminRoles,
		Groups:        groups,
		AdminSessions: adminSessions,
		AdminAudit:    adminAudit,
		RealmSettings: realmSettings,
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

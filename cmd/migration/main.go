// Command migration applies schema changes and seeds local data.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/config"
	"github.com/ppa/itprj-0007-sso/internal/infra/database"
	"github.com/ppa/itprj-0007-sso/internal/pkg"
	"github.com/ppa/itprj-0007-sso/internal/repository"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "migration:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return errors.New("command is required")
	}
	command := args[0]
	switch command {
	case "rotate-master-key":
		return fmt.Errorf("%s remains out of scope for M3; no changes were made", command)
	case "rotate-key", "up", "down", "seed", "bootstrap":
	default:
		usage()
		return fmt.Errorf("unknown command %q", command)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	db, err := database.Open(ctx, cfg.Database.URL)
	if err != nil {
		return err
	}
	defer db.Close()

	switch command {
	case "up":
		return migrateUp(ctx, db)
	case "down":
		return migrateDown(ctx, db)
	case "bootstrap":
		email, password, err := bootstrapArgs(args[1:])
		if err != nil {
			return err
		}
		return bootstrap(ctx, cfg, db, email, password)
	case "seed":
		password, err := seedArgs(args[1:])
		if err != nil {
			return err
		}
		return seed(ctx, cfg, db, password)
	case "rotate-key":
		return rotateKey(ctx, cfg, db)
	}
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: migration <command> [flags]

commands:
  up                          apply all pending migrations
  down                        roll back the latest migration
  bootstrap --email <email> --password-stdin
                              ensure default realm, platform-admin role, admin user, signing key
  seed --password-stdin       ensure sample realm data, two clients, and a sample user
  rotate-key                  create a new active signing key and retire the old one after 24h
  rotate-master-key           reserved for a later milestone`)
}

func migrateUp(ctx context.Context, db *database.DB) error {
	if err := db.Up(ctx); err != nil {
		return err
	}
	version, err := db.Version(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("migration: up applied, version=%d\n", version)
	return nil
}

func migrateDown(ctx context.Context, db *database.DB) error {
	if err := db.Down(ctx); err != nil {
		return err
	}
	version, err := db.Version(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("migration: down applied, version=%d\n", version)
	return nil
}

func bootstrapArgs(args []string) (string, string, error) {
	flags := flag.NewFlagSet("bootstrap", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	email := flags.String("email", "", "admin email")
	passwordStdin := flags.Bool("password-stdin", false, "read the password from stdin")
	if err := flags.Parse(args); err != nil {
		return "", "", err
	}
	if strings.TrimSpace(*email) == "" || *email != strings.TrimSpace(*email) {
		return "", "", errors.New("--email is required and must be trimmed")
	}
	if !*passwordStdin {
		return "", "", errors.New("--password-stdin is required; there is no default password")
	}
	password, err := readPassword()
	if err != nil {
		return "", "", err
	}
	return *email, password, nil
}

func seedArgs(args []string) (string, error) {
	flags := flag.NewFlagSet("seed", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	passwordStdin := flags.Bool("password-stdin", false, "read the sample user password from stdin")
	if err := flags.Parse(args); err != nil {
		return "", err
	}
	if !*passwordStdin {
		return "", errors.New("--password-stdin is required; there is no default password")
	}
	return readPassword()
}

func readPassword() (string, error) {
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 4096))
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	password := strings.TrimRight(string(raw), "\r\n")
	if len(password) < 12 {
		return "", fmt.Errorf("password must be at least 12 characters, got %d", len(password))
	}
	return password, nil
}

func bootstrap(ctx context.Context, cfg *config.Config, db *database.DB, email, password string) error {
	ring, err := pkg.NewKeyring(cfg.MasterKeys.Keys, cfg.MasterKeys.KeyID)
	if err != nil {
		return fmt.Errorf("build master keyring: %w", err)
	}

	realm, err := ensureRealm(ctx, cfg, db, domain.DefaultRealmName)
	if err != nil {
		return err
	}
	role, err := ensureRole(ctx, db, realm, domain.PlatformAdminRole, "Platform administrator")
	if err != nil {
		return err
	}

	users := repository.NewUserRepository(db.Pool)
	credentials := repository.NewCredentialRepository(db.Pool)
	user, err := users.GetByEmail(ctx, realm.ID, email)
	created := false
	if errors.Is(err, domain.ErrNotFound) {
		now := time.Now().UTC()
		user = &domain.User{
			ID:              domain.UserID(uuid.NewString()),
			RealmID:         realm.ID,
			Email:           email,
			Name:            "Platform Admin",
			EmailVerifiedAt: &now,
		}
		if err := users.Create(ctx, user); err != nil {
			return fmt.Errorf("create admin user: %w", err)
		}
		created = true
	} else if err != nil {
		return err
	}

	if _, err := credentials.GetByUserID(ctx, user.ID); errors.Is(err, domain.ErrNotFound) {
		if err := setPassword(ctx, credentials, user.ID, password); err != nil {
			return err
		}
		if !created {
			fmt.Println("bootstrap: admin credential was missing and has been set")
		}
	} else if err != nil {
		return err
	}

	roles := repository.NewRoleRepository(db.Pool)
	if err := roles.AssignToUser(ctx, realm.ID, user.ID, role.ID); err != nil {
		return fmt.Errorf("assign platform-admin: %w", err)
	}

	keys := repository.NewSigningKeyRepository(db.Pool, ring)
	active, err := keys.ListActive(ctx, realm.ID)
	if err != nil {
		return err
	}
	if len(active) == 0 {
		key, err := repository.GenerateSigningKey(ring, realm.ID)
		if err != nil {
			return err
		}
		if err := keys.Create(ctx, key); err != nil {
			return fmt.Errorf("store signing key: %w", err)
		}
		fmt.Printf("bootstrap: created signing key kid=%s\n", key.KID)
	}

	if created {
		fmt.Printf("bootstrap: created admin user id=%s\n", user.ID)
	} else {
		fmt.Printf("bootstrap: admin user already exists id=%s\n", user.ID)
	}
	fmt.Printf("bootstrap: realm=%s ready\n", realm.Name)
	return nil
}

func seed(ctx context.Context, cfg *config.Config, db *database.DB, password string) error {
	realm, err := ensureRealm(ctx, cfg, db, domain.DefaultRealmName)
	if err != nil {
		return err
	}
	role, err := ensureRole(ctx, db, realm, domain.MemberRole, "Realm member")
	if err != nil {
		return err
	}

	users := repository.NewUserRepository(db.Pool)
	credentials := repository.NewCredentialRepository(db.Pool)
	const sampleEmail = "user@example.com"
	user, err := users.GetByEmail(ctx, realm.ID, sampleEmail)
	created := false
	if errors.Is(err, domain.ErrNotFound) {
		now := time.Now().UTC()
		user = &domain.User{
			ID:              domain.UserID(uuid.NewString()),
			RealmID:         realm.ID,
			Email:           sampleEmail,
			Name:            "Sample User",
			GivenName:       "Sample",
			FamilyName:      "User",
			EmailVerifiedAt: &now,
		}
		if err := users.Create(ctx, user); err != nil {
			return fmt.Errorf("create sample user: %w", err)
		}
		created = true
	} else if err != nil {
		return err
	}

	if _, err := credentials.GetByUserID(ctx, user.ID); errors.Is(err, domain.ErrNotFound) {
		if err := setPassword(ctx, credentials, user.ID, password); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	roles := repository.NewRoleRepository(db.Pool)
	if err := roles.AssignToUser(ctx, realm.ID, user.ID, role.ID); err != nil {
		return fmt.Errorf("assign member role: %w", err)
	}
	if created {
		fmt.Printf("seed: created sample user id=%s\n", user.ID)
	} else {
		fmt.Printf("seed: sample user already exists id=%s\n", user.ID)
	}

	clients := repository.NewClientRepository(db.Pool)
	if err := seedPublicClient(ctx, realm, clients); err != nil {
		return err
	}
	if err := seedConfidentialClient(ctx, realm, clients); err != nil {
		return err
	}
	fmt.Printf("seed: realm=%s ready\n", realm.Name)
	return nil
}

func seedPublicClient(ctx context.Context, realm *domain.Realm, clients *repository.PgClientRepository) error {
	const clientID = "sample-spa"
	if _, err := clients.GetByClientID(ctx, realm.ID, clientID); err == nil {
		fmt.Printf("seed: client %s already exists\n", clientID)
		return nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	client := &domain.Client{
		ID:            domain.ClientID(uuid.NewString()),
		RealmID:       realm.ID,
		ClientID:      clientID,
		Type:          domain.ClientTypePublic,
		GrantTypes:    []string{"authorization_code", "refresh_token"},
		AllowedScopes: []string{domain.ScopeOpenID, domain.ScopeProfile, domain.ScopeEmail, domain.ScopeOfflineAccess},
	}
	redirects := []domain.ClientRedirectURI{
		{URI: "http://localhost:5173/callback", Kind: domain.RedirectKindAuthorize},
		{URI: "http://localhost:5173/", Kind: domain.RedirectKindPostLogout},
	}
	origins := []domain.ClientCORSOrigin{{Origin: "http://localhost:5173"}}
	if err := clients.Create(ctx, client, nil, redirects, origins); err != nil {
		return fmt.Errorf("create %s: %w", clientID, err)
	}
	fmt.Printf("seed: created public client %s\n", clientID)
	return nil
}

func seedConfidentialClient(ctx context.Context, realm *domain.Realm, clients *repository.PgClientRepository) error {
	const clientID = "sample-backend"
	if _, err := clients.GetByClientID(ctx, realm.ID, clientID); err == nil {
		fmt.Printf("seed: client %s already exists, secret unchanged\n", clientID)
		return nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	secret, err := pkg.RandomToken(32)
	if err != nil {
		return fmt.Errorf("generate client secret: %w", err)
	}
	client := &domain.Client{
		ID:            domain.ClientID(uuid.NewString()),
		RealmID:       realm.ID,
		ClientID:      clientID,
		Type:          domain.ClientTypeConfidential,
		GrantTypes:    []string{"authorization_code", "refresh_token", "client_credentials"},
		AllowedScopes: []string{domain.ScopeOpenID, domain.ScopeProfile, domain.ScopeEmail, domain.ScopeOfflineAccess},
	}
	redirects := []domain.ClientRedirectURI{
		{URI: "http://localhost:8080/callback", Kind: domain.RedirectKindAuthorize},
		{URI: "http://localhost:8080/", Kind: domain.RedirectKindPostLogout},
	}
	origins := []domain.ClientCORSOrigin{{Origin: "http://localhost:8080"}}
	if err := clients.Create(ctx, client, &domain.ClientSecret{
		ClientID:   client.ID,
		SecretHash: sha256Hex(secret),
	}, redirects, origins); err != nil {
		return fmt.Errorf("create %s: %w", clientID, err)
	}
	fmt.Printf("seed: created confidential client %s\n", clientID)
	fmt.Printf("seed: dev-only client secret (shown once): %s\n", secret)
	return nil
}

func ensureRealm(ctx context.Context, cfg *config.Config, db *database.DB, name string) (*domain.Realm, error) {
	realms := repository.NewRealmRepository(db.Pool)
	realm, err := realms.GetByName(ctx, name)
	if err == nil {
		return realm, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	realm = &domain.Realm{
		ID:       domain.RealmID(uuid.NewString()),
		Name:     name,
		Issuer:   strings.TrimRight(cfg.HTTP.ExternalURL, "/") + "/realms/" + name,
		Settings: []byte("{}"),
	}
	if err := realms.Create(ctx, realm); err != nil {
		return nil, fmt.Errorf("create realm %s: %w", name, err)
	}
	fmt.Printf("migration: created realm %s\n", name)
	return realm, nil
}

func ensureRole(ctx context.Context, db *database.DB, realm *domain.Realm, name, description string) (*domain.Role, error) {
	roles := repository.NewRoleRepository(db.Pool)
	role, err := roles.GetByName(ctx, realm.ID, name)
	if err == nil {
		return role, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	role = &domain.Role{
		ID:          domain.RoleID(uuid.NewString()),
		RealmID:     realm.ID,
		Name:        name,
		Description: description,
	}
	if err := roles.Create(ctx, role); err != nil {
		return nil, fmt.Errorf("create role %s: %w", name, err)
	}
	return role, nil
}

func setPassword(ctx context.Context, credentials *repository.PgCredentialRepository, userID domain.UserID, password string) error {
	hash, err := pkg.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return credentials.Upsert(ctx, &domain.Credential{
		UserID:       userID,
		PasswordHash: hash,
		Algorithm:    "argon2id",
		Params:       []byte(pkg.Argon2ParamsJSON(pkg.DefaultArgon2Params)),
	})
}

func rotateKey(ctx context.Context, cfg *config.Config, db *database.DB) error {
	ring, err := pkg.NewKeyring(cfg.MasterKeys.Keys, cfg.MasterKeys.KeyID)
	if err != nil {
		return fmt.Errorf("build master keyring: %w", err)
	}
	realms := repository.NewRealmRepository(db.Pool)
	realm, err := realms.GetByName(ctx, domain.DefaultRealmName)
	if err != nil {
		return fmt.Errorf("load default realm: %w", err)
	}
	keys := repository.NewSigningKeyRepository(db.Pool, ring)
	next, err := keys.RotateKey(ctx, realm.ID, 24*time.Hour)
	if err != nil {
		return fmt.Errorf("rotate signing key: %w", err)
	}
	audit := &domain.AuditEvent{
		ID:        domain.AuditEventID(uuid.NewString()),
		RealmID:   realm.ID,
		ActorType: domain.ActorTypeSystem,
		Type:      domain.AuditKeyRotated,
		Metadata:  []byte(fmt.Sprintf(`{"kid":%q}`, next.KID)),
	}
	if err := repository.NewAuditRepository(db.Pool).Insert(ctx, audit); err != nil {
		return fmt.Errorf("audit key rotation: %w", err)
	}
	published, err := keys.ListPublished(ctx, realm.ID)
	if err != nil {
		return err
	}
	fmt.Printf("rotation: new kid=%s realm=%s published=%d retire_after=24h\n",
		next.KID, realm.Name, len(published))
	return nil
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

package httphandler

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/pkg"
	"github.com/ppa/itprj-0007-sso/internal/repository"
)

func TestIntegrationMasterKeyRotationKeepsSecrets(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	it.ensureSigningKey(t, realm)
	ctx := context.Background()

	email := "rotate-" + uuid.NewString() + "@example.com"
	user := it.createUser(t, realm, email, "correct horse battery staple", true)

	issuer := repository.NewJWTIssuer(it.keys, it.ring)
	token, err := issuer.SignAccessToken(ctx, realm.ID, domain.AccessTokenClaims{
		Issuer: "issuer", Subject: string(user.ID), Audience: "client",
		ExpiresAt: time.Now().Add(time.Hour).Unix(), IssuedAt: time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	mfaSecret := "JBSWY3DPEHPK3PXP"
	encryptedMFA, err := it.ring.Encrypt([]byte(mfaSecret))
	if err != nil {
		t.Fatalf("encrypt MFA secret: %v", err)
	}
	if err := it.mfa.SaveTOTP(ctx, &domain.MFATOTP{
		UserID:          user.ID,
		SecretEncrypted: encryptedMFA,
	}); err != nil {
		t.Fatalf("save MFA secret: %v", err)
	}

	smtpPassword := "smtp-password"
	settings := domain.DefaultRealmSettings()
	encryptedSMTP, err := it.ring.Encrypt([]byte(smtpPassword))
	if err != nil {
		t.Fatalf("encrypt SMTP password: %v", err)
	}
	settings.SMTP.PasswordEncrypted = encryptedSMTP
	if _, err := it.realmSettings.UpdateSettings(ctx, realm.ID, settings, nil); err != nil {
		t.Fatalf("save realm settings: %v", err)
	}
	t.Cleanup(func() {
		_, _ = it.realmSettings.UpdateSettings(context.Background(), realm.ID,
			domain.DefaultRealmSettings(), nil)
	})

	newRing, err := pkg.NewKeyring(map[string][]byte{
		it.cfg.MasterKeys.KeyID: it.cfg.MasterKeys.Keys[it.cfg.MasterKeys.KeyID],
		"next":                  bytes.Repeat([]byte{9}, 32),
	}, "next")
	if err != nil {
		t.Fatalf("build rotated keyring: %v", err)
	}

	stats, err := repository.ReencryptMasterKeys(ctx, it.db.Pool, newRing)
	if err != nil {
		t.Fatalf("rotate master key: %v", err)
	}
	if stats.SigningKeys < 1 || stats.SMTPPasswords < 1 || stats.MFASecrets != 1 {
		t.Fatalf("rotation stats = %+v", stats)
	}

	key, err := it.keys.GetActive(ctx, realm.ID)
	if err != nil {
		t.Fatalf("read signing key: %v", err)
	}
	if kid, _ := newRing.KeyID(key.PrivateKeyEncrypted); kid != "next" {
		t.Fatalf("signing key still encrypted with %q", kid)
	}
	rotatedSettings, err := it.realmSettings.GetSettings(ctx, realm.ID)
	if err != nil {
		t.Fatalf("read realm settings: %v", err)
	}
	if kid, _ := newRing.KeyID(rotatedSettings.SMTP.PasswordEncrypted); kid != "next" {
		t.Fatalf("SMTP password still encrypted with %q", kid)
	}
	plainSMTP, err := newRing.Decrypt(rotatedSettings.SMTP.PasswordEncrypted)
	if err != nil || string(plainSMTP) != smtpPassword {
		t.Fatalf("SMTP password after rotation = %q, err = %v", plainSMTP, err)
	}
	rotatedMFA, err := it.mfa.GetTOTP(ctx, user.ID)
	if err != nil {
		t.Fatalf("read MFA secret: %v", err)
	}
	if kid, _ := newRing.KeyID(rotatedMFA.SecretEncrypted); kid != "next" {
		t.Fatalf("MFA secret still encrypted with %q", kid)
	}
	plainMFA, err := newRing.Decrypt(rotatedMFA.SecretEncrypted)
	if err != nil || string(plainMFA) != mfaSecret {
		t.Fatalf("MFA secret after rotation = %q, err = %v", plainMFA, err)
	}

	rotatedIssuer := repository.NewJWTIssuer(it.keys, newRing)
	if _, err := rotatedIssuer.VerifyAccessToken(ctx, realm.ID, token); err != nil {
		t.Fatalf("token signed before rotation stopped verifying: %v", err)
	}

	again, err := repository.ReencryptMasterKeys(ctx, it.db.Pool, newRing)
	if err != nil {
		t.Fatalf("second rotation: %v", err)
	}
	if again.SigningKeys != 0 || again.SMTPPasswords != 0 || again.MFASecrets != 0 {
		t.Fatalf("second rotation was not idempotent: %+v", again)
	}
}

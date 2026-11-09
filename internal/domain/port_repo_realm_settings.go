package domain

import "context"

// RealmSettingsRepository reads and writes realm.settings, including the
// encrypted SMTP password.
type RealmSettingsRepository interface {
	GetSettings(ctx context.Context, realmID RealmID) (*RealmSettings, error)
	UpdateSettings(ctx context.Context, realmID RealmID, settings *RealmSettings, audit *AuditEvent) (*RealmSettings, error)
}

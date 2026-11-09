package domain

import "context"

// GroupRepository owns flat groups and their membership. It has no role
// inheritance: membership is a direct user list.
type GroupRepository interface {
	List(ctx context.Context, realmID RealmID) ([]GroupView, error)
	Get(ctx context.Context, realmID RealmID, groupID GroupID) (*GroupView, error)
	Create(ctx context.Context, group *Group, audit *AuditEvent) error
	Update(ctx context.Context, realmID RealmID, groupID GroupID, update GroupUpdate, audit *AuditEvent) (*Group, error)
	Delete(ctx context.Context, realmID RealmID, groupID GroupID, audit *AuditEvent) error
	ListMembers(ctx context.Context, realmID RealmID, groupID GroupID) ([]GroupMember, error)
	AddMembers(ctx context.Context, realmID RealmID, groupID GroupID, userIDs []UserID, audit *AuditEvent) error
	RemoveMember(ctx context.Context, realmID RealmID, groupID GroupID, userID UserID, audit *AuditEvent) error
}

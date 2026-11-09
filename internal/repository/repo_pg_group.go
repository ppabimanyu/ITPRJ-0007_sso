package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/database"
)

// PgGroupRepository owns flat groups and their membership.
type PgGroupRepository struct {
	pool *pgxpool.Pool
}

func NewGroupRepository(pool *pgxpool.Pool) *PgGroupRepository {
	return &PgGroupRepository{pool: pool}
}

const groupViewColumns = `g.id::text, g.name, g.description,
	(SELECT count(*) FROM user_group ug WHERE ug.group_id = g.id)`

func (r *PgGroupRepository) List(ctx context.Context, realmID domain.RealmID) ([]domain.GroupView, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+groupViewColumns+`
		FROM "group" g WHERE g.realm_id = $1::text::uuid
		ORDER BY g.name`, string(realmID))
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	views := make([]domain.GroupView, 0)
	for rows.Next() {
		view, err := scanGroupView(rows)
		if err != nil {
			return nil, err
		}
		views = append(views, *view)
	}
	return views, mapError(rows.Err())
}

func (r *PgGroupRepository) Get(ctx context.Context, realmID domain.RealmID, groupID domain.GroupID) (*domain.GroupView, error) {
	return scanGroupView(r.pool.QueryRow(ctx, `
		SELECT `+groupViewColumns+`
		FROM "group" g
		WHERE g.realm_id = $1::text::uuid AND g.id = $2::text::uuid`,
		string(realmID), string(groupID)))
}

func (r *PgGroupRepository) Create(ctx context.Context, group *domain.Group, audit *domain.AuditEvent) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO "group" (id, realm_id, name, description)
			VALUES ($1::text::uuid, $2::text::uuid, $3, $4)
			RETURNING created_at`,
			string(group.ID), string(group.RealmID), group.Name, group.Description,
		).Scan(&group.CreatedAt)
		if err != nil {
			return mapError(err)
		}
		return insertAuditTx(ctx, tx, audit)
	})
}

func (r *PgGroupRepository) Update(ctx context.Context, realmID domain.RealmID, groupID domain.GroupID, update domain.GroupUpdate, audit *domain.AuditEvent) (*domain.Group, error) {
	group := &domain.Group{ID: groupID, RealmID: realmID}
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT id::text, realm_id::text, name, description, created_at
			FROM "group"
			WHERE realm_id = $1::text::uuid AND id = $2::text::uuid
			FOR UPDATE`, string(realmID), string(groupID),
		).Scan(&group.ID, &group.RealmID, &group.Name, &group.Description, &group.CreatedAt)
		if err != nil {
			return mapError(err)
		}
		if update.Name != nil {
			group.Name = *update.Name
		}
		if update.Description.Set {
			if update.Description.Value == nil {
				group.Description = ""
			} else {
				group.Description = *update.Description.Value
			}
		}
		_, err = tx.Exec(ctx, `
			UPDATE "group" SET name = $3, description = $4
			WHERE realm_id = $1::text::uuid AND id = $2::text::uuid`,
			string(realmID), string(groupID), group.Name, group.Description)
		if err != nil {
			return mapError(err)
		}
		return insertAuditTx(ctx, tx, audit)
	})
	if err != nil {
		return nil, err
	}
	return group, nil
}

func (r *PgGroupRepository) Delete(ctx context.Context, realmID domain.RealmID, groupID domain.GroupID, audit *domain.AuditEvent) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		var found string
		err := tx.QueryRow(ctx, `
			SELECT id::text FROM "group"
			WHERE realm_id = $1::text::uuid AND id = $2::text::uuid
			FOR UPDATE`, string(realmID), string(groupID)).Scan(&found)
		if err != nil {
			return mapError(err)
		}
		if err := insertAuditTx(ctx, tx, audit); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM "group" WHERE id = $1::text::uuid`, string(groupID))
		return mapError(err)
	})
}

func (r *PgGroupRepository) ListMembers(ctx context.Context, realmID domain.RealmID, groupID domain.GroupID) ([]domain.GroupMember, error) {
	if err := r.groupExists(ctx, realmID, groupID); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT u.id::text, u.email, u.name
		FROM user_group ug
		JOIN "user" u ON u.id = ug.user_id
		WHERE ug.realm_id = $1::text::uuid AND ug.group_id = $2::text::uuid
		ORDER BY u.email`, string(realmID), string(groupID))
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	members := make([]domain.GroupMember, 0)
	for rows.Next() {
		var member domain.GroupMember
		var name string
		if err := rows.Scan(&member.UserID, &member.Email, &name); err != nil {
			return nil, mapError(err)
		}
		member.Name = optionalStringValue(name)
		members = append(members, member)
	}
	return members, mapError(rows.Err())
}

func (r *PgGroupRepository) AddMembers(ctx context.Context, realmID domain.RealmID, groupID domain.GroupID, userIDs []domain.UserID, audit *domain.AuditEvent) error {
	ids := uniqueUserIDs(userIDs)
	if len(ids) == 0 {
		return domain.ErrNotFound
	}
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := groupExistsTx(ctx, tx, realmID, groupID); err != nil {
			return err
		}
		var found int
		err := tx.QueryRow(ctx, `
			SELECT count(*) FROM "user"
			WHERE realm_id = $1::text::uuid AND id::text = ANY($2::text[])`,
			string(realmID), ids).Scan(&found)
		if err != nil {
			return mapError(err)
		}
		if found != len(ids) {
			return domain.ErrNotFound
		}
		for _, userID := range ids {
			if _, err := tx.Exec(ctx, `
				INSERT INTO user_group (realm_id, group_id, user_id)
				VALUES ($1::text::uuid, $2::text::uuid, $3::text::uuid)
				ON CONFLICT (group_id, user_id) DO NOTHING`,
				string(realmID), string(groupID), userID); err != nil {
				return mapError(err)
			}
		}
		return insertAuditTx(ctx, tx, audit)
	})
}

func (r *PgGroupRepository) RemoveMember(ctx context.Context, realmID domain.RealmID, groupID domain.GroupID, userID domain.UserID, audit *domain.AuditEvent) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := groupExistsTx(ctx, tx, realmID, groupID); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			DELETE FROM user_group
			WHERE realm_id = $1::text::uuid AND group_id = $2::text::uuid
				AND user_id = $3::text::uuid`,
			string(realmID), string(groupID), string(userID))
		if err != nil {
			return mapError(err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		return insertAuditTx(ctx, tx, audit)
	})
}

func (r *PgGroupRepository) groupExists(ctx context.Context, realmID domain.RealmID, groupID domain.GroupID) error {
	var found string
	err := r.pool.QueryRow(ctx, `
		SELECT id::text FROM "group"
		WHERE realm_id = $1::text::uuid AND id = $2::text::uuid`,
		string(realmID), string(groupID)).Scan(&found)
	return mapError(err)
}

func groupExistsTx(ctx context.Context, tx pgx.Tx, realmID domain.RealmID, groupID domain.GroupID) error {
	var found string
	err := tx.QueryRow(ctx, `
		SELECT id::text FROM "group"
		WHERE realm_id = $1::text::uuid AND id = $2::text::uuid
		FOR UPDATE`, string(realmID), string(groupID)).Scan(&found)
	return mapError(err)
}

func uniqueUserIDs(userIDs []domain.UserID) []string {
	seen := make(map[string]struct{}, len(userIDs))
	ids := make([]string, 0, len(userIDs))
	for _, userID := range userIDs {
		value := string(userID)
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		ids = append(ids, value)
	}
	return ids
}

func scanGroupView(row rowScanner) (*domain.GroupView, error) {
	var view domain.GroupView
	var description string
	var memberCount int64
	if err := row.Scan(&view.ID, &view.Name, &description, &memberCount); err != nil {
		return nil, mapError(err)
	}
	view.Description = optionalStringValue(description)
	view.MemberCount = int(memberCount)
	return &view, nil
}

var _ domain.GroupRepository = (*PgGroupRepository)(nil)

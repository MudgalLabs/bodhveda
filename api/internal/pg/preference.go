package pg

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
	tantraRepo "github.com/mudgallabs/tantra/repository"
)

type PreferenceRepo struct {
	db   dbx.DBExecutor
	pool *pgxpool.Pool
}

func NewPreferenceRepo(db *pgxpool.Pool) repository.PreferenceRepository {
	return &PreferenceRepo{
		db:   db,
		pool: db,
	}
}

func (r *PreferenceRepo) Create(ctx context.Context, pref *entity.Preference) (*entity.Preference, error) {
	sql := `
		INSERT INTO preference (project_id, recipient_id, channel, topic, event, label, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, project_id, recipient_id, channel, topic, event, label, enabled, created_at, updated_at
	`

	row := r.db.QueryRow(ctx, sql, pref.ProjectID, pref.RecipientID, pref.Channel, pref.Topic, pref.Event, pref.Label, pref.Enabled, pref.CreatedAt, pref.UpdatedAt)

	var newPref entity.Preference

	err := row.Scan(&newPref.ID, &newPref.ProjectID, &newPref.RecipientID, &newPref.Channel, &newPref.Topic, &newPref.Event, &newPref.Label, &newPref.Enabled, &newPref.CreatedAt, &newPref.UpdatedAt)
	if err != nil {
		if dbx.IsUniqueViolation(err) {
			return nil, tantraRepo.ErrConflict
		}
		return nil, err
	}

	return &newPref, nil
}

func (r *PreferenceRepo) ListProjectPreferences(ctx context.Context, projectID int) ([]*entity.Preference, error) {
	sql := `
		SELECT id, project_id, recipient_id, channel, topic, event, label, enabled, created_at, updated_at
		FROM preference
		WHERE project_id = $1
		ORDER BY channel, topic NULLS FIRST, event NULLS FIRST, label ASC
	`

	rows, err := r.db.Query(ctx, sql, projectID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	prefs := []*entity.Preference{}
	for rows.Next() {
		var newPref entity.Preference
		err := rows.Scan(&newPref.ID, &newPref.ProjectID, &newPref.RecipientID, &newPref.Channel, &newPref.Topic, &newPref.Event, &newPref.Label, &newPref.Enabled, &newPref.CreatedAt, &newPref.UpdatedAt)

		if err != nil {
			return nil, err
		}

		prefs = append(prefs, &newPref)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return prefs, nil
}

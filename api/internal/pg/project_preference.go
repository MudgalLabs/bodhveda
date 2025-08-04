package pg

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
)

type ProjectPreferenceRepo struct {
	db   dbx.DBExecutor
	pool *pgxpool.Pool
}

func NewProjectPreferenceRepo(db *pgxpool.Pool) repository.ProjectPreferenceRepository {
	return &ProjectPreferenceRepo{
		db:   db,
		pool: db,
	}
}

func (r *ProjectPreferenceRepo) Create(ctx context.Context, pref *entity.ProjectPreference) (*entity.ProjectPreference, error) {
	sql := `
		INSERT INTO project_preference (project_id, channel, topic, event, label, default_enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, project_id, channel, topic, event, label, default_enabled, created_at, updated_at
	`
	now := time.Now().UTC()

	row := r.db.QueryRow(ctx, sql, pref.ProjectID, pref.Channel, pref.Topic, pref.Event, pref.Label, pref.DefaultEnabled, now, now)

	var newPref entity.ProjectPreference
	err := row.Scan(&newPref.ID, &newPref.ProjectID, &newPref.Channel, &newPref.Topic, &newPref.Event, &newPref.Label, &newPref.DefaultEnabled, &newPref.CreatedAt, &newPref.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &newPref, nil
}

func (r *ProjectPreferenceRepo) List(ctx context.Context, projectID int) ([]*entity.ProjectPreference, error) {
	sql := `
		SELECT id, project_id, channel, topic, event, label, default_enabled, created_at, updated_at
		FROM project_preference
		WHERE project_id = $1
		ORDER BY channel, topic NULLS FIRST, event NULLS FIRST, label ASC
	`

	rows, err := r.db.Query(ctx, sql, projectID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	prefs := []*entity.ProjectPreference{}
	for rows.Next() {
		var newPref entity.ProjectPreference
		err := rows.Scan(&newPref.ID, &newPref.ProjectID, &newPref.Channel, &newPref.Topic, &newPref.Event, &newPref.Label, &newPref.DefaultEnabled, &newPref.CreatedAt, &newPref.UpdatedAt)

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

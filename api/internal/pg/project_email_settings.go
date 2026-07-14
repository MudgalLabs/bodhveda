package pg

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
	tantraRepo "github.com/mudgallabs/tantra/repository"
)

type ProjectEmailSettingsRepo struct {
	db dbx.DBExecutor
}

func NewProjectEmailSettingsRepo(db *pgxpool.Pool) repository.ProjectEmailSettingsRepository {
	return &ProjectEmailSettingsRepo{
		db: db,
	}
}

const projectEmailSettingsFields = `
	project_id, provider, secret, nonce, from_name, from_address, webhook_secret, webhook_nonce, created_at, updated_at
`

func scanProjectEmailSettings(row interface {
	Scan(dest ...any) error
}) (*entity.ProjectEmailSettings, error) {
	var s entity.ProjectEmailSettings
	var provider string
	err := row.Scan(&s.ProjectID, &provider, &s.Secret, &s.Nonce, &s.FromName, &s.FromAddress,
		&s.WebhookSecret, &s.WebhookNonce, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	s.Provider = enum.EmailProvider(provider)
	return &s, nil
}

func (r *ProjectEmailSettingsRepo) Get(ctx context.Context, projectID int) (*entity.ProjectEmailSettings, error) {
	sql := `
		SELECT ` + projectEmailSettingsFields + `
		FROM project_email_settings
		WHERE project_id = $1
	`

	row := r.db.QueryRow(ctx, sql, projectID)
	settings, err := scanProjectEmailSettings(row)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, tantraRepo.ErrNotFound
		}
		return nil, err
	}

	return settings, nil
}

func (r *ProjectEmailSettingsRepo) Upsert(ctx context.Context, s *entity.ProjectEmailSettings) (*entity.ProjectEmailSettings, error) {
	sql := `
		INSERT INTO project_email_settings
			(project_id, provider, secret, nonce, from_name, from_address, webhook_secret, webhook_nonce, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (project_id) DO UPDATE SET
			provider = EXCLUDED.provider,
			secret = EXCLUDED.secret,
			nonce = EXCLUDED.nonce,
			from_name = EXCLUDED.from_name,
			from_address = EXCLUDED.from_address,
			webhook_secret = EXCLUDED.webhook_secret,
			webhook_nonce = EXCLUDED.webhook_nonce,
			updated_at = EXCLUDED.updated_at
		RETURNING ` + projectEmailSettingsFields + `
	`

	row := r.db.QueryRow(ctx, sql,
		s.ProjectID, string(s.Provider), s.Secret, s.Nonce, s.FromName, s.FromAddress,
		s.WebhookSecret, s.WebhookNonce, s.CreatedAt, s.UpdatedAt,
	)

	return scanProjectEmailSettings(row)
}

package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
)

type ProjectEmailSettingsRepository interface {
	ProjectEmailSettingsReader
	ProjectEmailSettingsWriter
}

type ProjectEmailSettingsReader interface {
	// Get returns the project's email settings, or tantra repository.ErrNotFound
	// when the project has none configured yet.
	Get(ctx context.Context, projectID int) (*entity.ProjectEmailSettings, error)
	// GetTx is Get on the caller's transaction.
	GetTx(ctx context.Context, tx pgx.Tx, projectID int) (*entity.ProjectEmailSettings, error)
}

type ProjectEmailSettingsWriter interface {
	// Upsert inserts or replaces the project's email settings (one row per project).
	Upsert(ctx context.Context, settings *entity.ProjectEmailSettings) (*entity.ProjectEmailSettings, error)
}

package repository

import (
	"context"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
)

type PreferenceRepository interface {
	PreferenceReader
	PreferenceWriter
}

type PreferenceReader interface {
	ListProjectPreferences(ctx context.Context, projectID int) ([]*entity.Preference, error)
}

type PreferenceWriter interface {
	Create(ctx context.Context, pref *entity.Preference) (*entity.Preference, error)
}

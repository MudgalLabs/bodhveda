package repository

import (
	"context"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
)

type ProjectPreferenceRepository interface {
	ProjectPreferenceReader
	ProjectPreferenceWriter
}

type ProjectPreferenceReader interface {
	List(ctx context.Context, projectID int) ([]*entity.ProjectPreference, error)
}

type ProjectPreferenceWriter interface {
	Create(ctx context.Context, pref *entity.ProjectPreference) (*entity.ProjectPreference, error)
}

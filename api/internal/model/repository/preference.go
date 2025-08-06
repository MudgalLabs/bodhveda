package repository

import (
	"context"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/tantra/query"
)

type PreferenceRepository interface {
	PreferenceReader
	PreferenceWriter
}

type PreferenceReader interface {
	ListPreferences(ctx context.Context, projectID int, kind enum.PreferenceKind) ([]*entity.Preference, error)
}

type PreferenceWriter interface {
	Create(ctx context.Context, pref *entity.Preference) (*entity.Preference, error)
}

type PreferenceSearchFilter struct {
	ProjectOrRecipient enum.PreferenceKind
	ProjectID          *int // Only applied if ProjectOrRecipient is enum.PreferenceKindProject
}

type SearchPreferencePayload = query.SearchPayload[PreferenceSearchFilter]

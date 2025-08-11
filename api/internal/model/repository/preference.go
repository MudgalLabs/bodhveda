package repository

import (
	"context"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
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
	ShouldDirectNotificationBeDelivered(ctx context.Context, projectID int, recipientExtID string, target dto.Target) (bool, error)
	ListEligibleRecipientExtIDsForBroadcast(ctx context.Context, projectID int, target dto.Target) ([]string, error)
	ListPreferencesForRecipient(ctx context.Context, projectID int, recipientExtID string) ([]*entity.Preference, error)
}

type PreferenceWriter interface {
	Create(ctx context.Context, pref *entity.Preference) (*entity.Preference, error)
	Delete(ctx context.Context, projectID int, preferenceID int) error
	DeleteForRecipient(ctx context.Context, projectID int, recipientExtID string) (int, error)
	DeleteForProject(ctx context.Context, projectID int) (int, error)
}

type PreferenceSearchFilter struct {
	ProjectOrRecipient enum.PreferenceKind
	ProjectID          int
	RecipientExtID     *string
}

type SearchPreferencePayload = query.SearchPayload[PreferenceSearchFilter]

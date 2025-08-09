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
	ShouldDirectNotificationBeDelivered(ctx context.Context, projectID int, target dto.NotificationTarget) (bool, error)
	ListEligibleRecipientExtIDsForBroadcast(ctx context.Context, projectID int, target dto.NotificationTarget) ([]string, error)
	ListPreferencesForRecipient(ctx context.Context, projectID int, recipientExtID string) ([]*entity.Preference, error)
}

type PreferenceWriter interface {
	Create(ctx context.Context, pref *entity.Preference) (*entity.Preference, error)
	DeleteForRecipient(ctx context.Context, projectID int, recipientExtID string) error
}

type PreferenceSearchFilter struct {
	ProjectOrRecipient enum.PreferenceKind
	ProjectID          int
	RecipientExtID     *string
}

type SearchPreferencePayload = query.SearchPayload[PreferenceSearchFilter]

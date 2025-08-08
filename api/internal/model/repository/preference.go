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
}

type PreferenceWriter interface {
	Create(ctx context.Context, pref *entity.Preference) (*entity.Preference, error)
}

type PreferenceSearchFilter struct {
	ProjectOrRecipient enum.PreferenceKind
	ProjectID          int
}

type SearchPreferencePayload = query.SearchPayload[PreferenceSearchFilter]

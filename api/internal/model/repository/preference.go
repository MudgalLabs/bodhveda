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
	DoesProjectPreferenceExist(ctx context.Context, projectID int, target dto.Target, medium enum.Medium) (bool, error)
	ListPreferences(ctx context.Context, projectID int, kind enum.PreferenceKind) ([]*entity.Preference, error)
	ShouldDirectNotificationBeDelivered(ctx context.Context, projectID int, recipientExtID string, target dto.Target, medium enum.Medium) (bool, error)
	ListEligibleRecipientExtIDsForBroadcast(ctx context.Context, projectID int, target dto.Target, medium enum.Medium) ([]string, error)
	// ResolveRecipientPreferences answers every known (target, medium) for one
	// recipient with the SAME cascade ShouldDirectNotificationBeDelivered uses,
	// in one query. Callers pass the mediums to resolve (see enum.ActiveMediums).
	ResolveRecipientPreferences(ctx context.Context, projectID int, recipientExtID string, mediums []enum.Medium) ([]*entity.ResolvedPreference, error)
	// ResolveRecipientPreferenceForTargets runs that same cascade over exactly
	// the targets given, including ones nothing is stored about — which is why a
	// single-target check cannot just filter ResolveRecipientPreferences.
	ResolveRecipientPreferenceForTargets(ctx context.Context, projectID int, recipientExtID string, mediums []enum.Medium, targets []dto.Target) ([]*entity.ResolvedPreference, error)
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
}

type SearchPreferencePayload = query.SearchPayload[PreferenceSearchFilter]

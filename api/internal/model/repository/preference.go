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
	// GetProjectPreferenceByID fetches a single catalog entry (a project-level
	// row) by id, scoped to the project. It returns tantra's ErrNotFound when no
	// project-level row with that id exists — a recipient-level row with the same
	// id is invisible here, so a full-scope key cannot read one through the
	// catalog surface.
	GetProjectPreferenceByID(ctx context.Context, projectID int, preferenceID int) (*entity.Preference, error)
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
	// UpdateProjectPreference updates a catalog entry's mutable fields (name,
	// description and the project-level default). Scoped to project-level rows
	// (recipient NULL) and to the project; returns tantra's ErrNotFound when no
	// such row exists. A nil description clears the entry's description.
	UpdateProjectPreference(ctx context.Context, projectID int, preferenceID int, name string, description *string, enabled bool) (*entity.Preference, error)
	// UpsertProjectPreferences declaratively merges a set of catalog entries in a
	// single transaction: each is upserted by its natural key (channel, topic,
	// event, medium) — inserted if new, its name + description + default updated if
	// it exists.
	// When prune is false (the default, merge) catalog rows NOT in the set are
	// left untouched; when prune is true they are also deleted, making the set the
	// project's entire desired catalog. Returns the full resulting project-level
	// catalog. Recipient rows are never touched.
	UpsertProjectPreferences(ctx context.Context, projectID int, prefs []*entity.Preference, prune bool) ([]*entity.Preference, error)
	// DeleteProjectPreference removes a catalog entry (a project-level row) by id,
	// scoped to the project. Like GetProjectPreferenceByID it is confined to
	// project-level rows, so a full-scope key deleting through the catalog surface
	// cannot un-set a recipient's own preference by id. Returns ErrNotFound when
	// no project-level row with that id exists.
	DeleteProjectPreference(ctx context.Context, projectID int, preferenceID int) error
	Delete(ctx context.Context, projectID int, preferenceID int) error
	DeleteForRecipient(ctx context.Context, projectID int, recipientExtID string) (int, error)
	DeleteForProject(ctx context.Context, projectID int) (int, error)
}

type PreferenceSearchFilter struct {
	ProjectOrRecipient enum.PreferenceKind
	ProjectID          int
}

type SearchPreferencePayload = query.SearchPayload[PreferenceSearchFilter]

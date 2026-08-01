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
	// LookupCatalogEntry resolves a (target, medium) against the catalog with the
	// SAME wildcard rules the delivery cascade uses (a topic='any' row matches a
	// concrete topic, except when the topic is literally 'none'), and reports
	// whether the matched entry is mandatory. It is the strict-target gate's
	// primitive — see the implementation for why exact match would break Grahak.
	LookupCatalogEntry(ctx context.Context, projectID int, target dto.Target, medium enum.Medium) (exists bool, mandatory bool, err error)
	ListPreferences(ctx context.Context, projectID int, kind enum.PreferenceKind) ([]*entity.Preference, error)
	// GetProjectPreferenceByID fetches a single catalog entry (a project-level
	// row) by id, scoped to the project. It returns tantra's ErrNotFound when no
	// project-level row with that id exists — a recipient-level row with the same
	// id is invisible here, so a full-scope key cannot read one through the
	// catalog surface.
	GetProjectPreferenceByID(ctx context.Context, projectID int, preferenceID int) (*entity.Preference, error)
	ShouldDirectNotificationBeDelivered(ctx context.Context, projectID int, recipientExtID string, target dto.Target, medium enum.Medium) (bool, error)
	ListEligibleRecipientExtIDsForBroadcast(ctx context.Context, projectID int, target dto.Target, medium enum.Medium) ([]string, error)

	// FilterEligibleRecipientsForBroadcast narrows a KNOWN candidate set to those
	// eligible on `medium`. Broadcast email resolves eligibility per batch, and
	// the project-wide list can dwarf the batch — see the implementation.
	FilterEligibleRecipientsForBroadcast(ctx context.Context, projectID int, target dto.Target, medium enum.Medium, recipientExtIDs []string) ([]string, error)
	// CountBroadcastAudience returns the recipient breakdown for a target: total,
	// eligible, and the two DIFFERENT reasons a recipient is excluded (their own
	// opt-out vs the project never cataloging the target).
	//
	// ⚠️ Its eligibility predicate MUST stay identical to
	// ListEligibleRecipientExtIDsForBroadcast's — they are two views of one rule,
	// and a drift between them shows up as a tree whose numbers do not add up.
	// pg keeps them adjacent and cross-checks them in a test for that reason.
	CountBroadcastAudience(ctx context.Context, projectID int, target dto.Target, medium enum.Medium) (*entity.BroadcastAudience, error)
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

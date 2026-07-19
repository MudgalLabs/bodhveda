package entity

import (
	"time"
)

type Preference struct {
	ID             int
	ProjectID      *int    // Nullable, if null then this is a recipient preference.
	RecipientExtID *string // Nullable, if null then this is a project preference.
	Channel        string
	// Topic can be "none", "any", or a specific string.
	// Meaning that "any" and "none" are reserved keywords for the system.
	// So app developers should not use these values for their own topics.
	// `any` means the preference applies to all topics in the channel.
	// Ex: Comment on your post (channel="posts", topic="any", event="new_comment").
	// `none` means this rule does not have any topic.
	// Ex: Announcements for new features (channel="annoucements", topic="none", event="new_feature").
	Topic string
	Event string
	// Medium is the delivery transport this preference gates (in_app, email, ...).
	// Legacy rows backfill to "in_app". A project-level (recipient NULL) row is a
	// catalog entry declaring that (target, medium) may fire.
	Medium  string
	Enabled bool
	// Name is the catalog entry's human name (e.g. "Marketing emails"). Nullable:
	// null on a recipient-level row, required on a project-level (catalog) row.
	Name *string
	// Description is an optional longer blurb for a catalog entry
	// (e.g. "Receive notifications about new products, features, and more.").
	// Nullable and, like Name, only ever set on project-level rows.
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewPreference(projectID *int, recipientExtID *string, channel string, topic string, event string, medium string, name *string, description *string, enabled bool) *Preference {
	now := time.Now().UTC()

	return &Preference{
		ProjectID:      projectID,
		RecipientExtID: recipientExtID,
		Channel:        channel,
		Topic:          topic,
		Event:          event,
		Medium:         medium,
		Name:           name,
		Description:    description,
		Enabled:        enabled,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

type ProjectPreferenceListItem struct {
	Preference

	Subscribers int
}

// PreferenceSource names which rung of the resolution cascade decided a
// resolved preference. It exists so the console can explain *why* a cell reads
// the way it does, rather than only what it reads.
type PreferenceSource string

const (
	// PreferenceSourceRecipientExact — the recipient's own row for this exact
	// (target, medium). The only source a toggle writes directly.
	PreferenceSourceRecipientExact PreferenceSource = "recipient_exact"
	// PreferenceSourceRecipientAny — the recipient's own topic='any' rule for
	// this channel/event.
	PreferenceSourceRecipientAny PreferenceSource = "recipient_any"
	// PreferenceSourceProjectExact — the project catalog's row for this exact
	// (target, medium).
	PreferenceSourceProjectExact PreferenceSource = "project_exact"
	// PreferenceSourceProjectAny — the project catalog's topic='any' rule.
	PreferenceSourceProjectAny PreferenceSource = "project_any"
	// PreferenceSourceDefault — nothing matched, so the medium-dependent default
	// decided: in_app delivers, every other medium does not.
	PreferenceSourceDefault PreferenceSource = "default"
)

// ResolvedPreference is what a single (target, medium) ACTUALLY resolves to for
// one recipient — the same answer the send path's gating cascade would give.
//
// It is deliberately not a stored row. A recipient with no row of their own is
// not "unset": they follow the project default, and for in_app that default is
// DELIVER. Equally, `Cataloged` is not a gate — an explicit recipient row on an
// uncataloged (target, medium) still delivers, because it wins the cascade
// before the catalog is ever consulted. Only Enabled is the honest answer;
// Cataloged and Source are context for explaining it.
type ResolvedPreference struct {
	Channel string
	Topic   string
	Event   string
	Medium string
	// Name is the catalog entry's human name, when this (target, medium) is
	// cataloged. Nil otherwise.
	Name *string
	// Description is the catalog entry's optional longer blurb, when this
	// (target, medium) is cataloged and has one. Nil otherwise.
	Description *string
	// Enabled is the resolved delivery decision — what a send would do.
	Enabled bool
	// Cataloged reports whether a project-level row exists for this exact
	// (target, medium). Context only; it does not gate Enabled.
	Cataloged bool
	// Source names the cascade rung that decided Enabled.
	Source PreferenceSource
}

// Inherited reports whether the recipient has no row of their own for this exact
// (target, medium) — i.e. the resolved value came from anywhere else in the
// cascade. Toggling the cell writes exactly that missing row.
func (r ResolvedPreference) Inherited() bool {
	return r.Source != PreferenceSourceRecipientExact
}

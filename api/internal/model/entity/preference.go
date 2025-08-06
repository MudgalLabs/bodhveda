package entity

import (
	"time"
)

type Preference struct {
	ID          int
	ProjectID   *int // Nullable, if null then this is a recipient preference.
	RecipientID *int // Nullable, if null then this is a project preference.
	Channel     string
	// Topic can be "none", "any", or a specific string.
	// Meaning that "any" and "none" are reserved keywords for the system.
	// So app developers should not use these values for their own topics.
	// `any` means the preference applies to all topics in the channel.
	// Ex: Comment on your post (channel="posts", topic="any", event="new_comment").
	// `none` means this rule does not have any topic.
	// Ex: Announcements for new features (channel="annoucements", topic="none", event="new_feature").
	Topic     string
	Event     string
	Enabled   bool
	Label     *string // Nullable, if null then this is a recipient preference.
	CreatedAt time.Time
	UpdatedAt time.Time

	// Extra fields for convenience.
	// These fields are not stored in the database.
	RecipientExtID *string // Nullable, if null then this is a project preference.
}

func NewPreference(projectID *int, recipientID *int, channel string, topic string, event string, label *string, enabled bool) *Preference {
	now := time.Now().UTC()

	return &Preference{
		ProjectID:   projectID,
		RecipientID: recipientID,
		Channel:     channel,
		Topic:       topic,
		Event:       event,
		Label:       label,
		Enabled:     enabled,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

package entity

import (
	"strings"
	"time"
)

type Recipient struct {
	ID         int
	ExternalID string // Unique recipient ID from the client's system.
	ProjectID  int
	Name       string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func NewRecipient(projectID int, externalID, name string) *Recipient {
	now := time.Now().UTC()
	return &Recipient{
		ExternalID: strings.ToLower(externalID),
		ProjectID:  projectID,
		Name:       name,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

type RecipientListItem struct {
	Recipient

	DirectNotificationsCount    int `json:"direct_notifications_count"`
	BroadcastNotificationsCount int `json:"broadcast_notifications_count"`
}

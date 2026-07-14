package entity

import (
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/enum"
)

// RecipientContact is a per-medium contact address for a recipient (e.g. an
// email address). A recipient may have multiple contacts per medium, at most one
// of which is the primary (enforced by a partial unique index). `in_app` is not
// a valid contact medium — see enum.Medium.
type RecipientContact struct {
	ID             int64
	ProjectID      int
	RecipientExtID string
	Medium         enum.Medium
	Address        string
	IsPrimary      bool
	VerifiedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewRecipientContact(projectID int, recipientExtID string, medium enum.Medium, address string, isPrimary bool) *RecipientContact {
	now := time.Now().UTC()
	return &RecipientContact{
		ProjectID:      projectID,
		RecipientExtID: recipientExtID,
		Medium:         medium,
		Address:        address,
		IsPrimary:      isPrimary,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

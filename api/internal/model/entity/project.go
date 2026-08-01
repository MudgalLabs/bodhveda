package entity

import (
	"time"
)

type Project struct {
	ID     int
	Name   string
	UserID int

	// StrictTargets makes the catalog a gateway: a send naming a (target, medium)
	// with no catalog entry is rejected with a 400 instead of being accepted.
	//
	// ⚠️ Defaults to FALSE, including for brand new projects, and that is a
	// deliberate product decision rather than a migration convenience. Strictness
	// is a maturity setting: with it on from the start, a user's first targeted
	// send fails before they know what a catalog is. See
	// NotificationService.gateTarget.
	StrictTargets bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewProject(userID int, name string) *Project {
	now := time.Now().UTC()
	return &Project{
		Name:      name,
		UserID:    userID,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

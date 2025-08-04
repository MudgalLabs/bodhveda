package entity

import "time"

type ProjectPreference struct {
	ID             int
	ProjectID      int
	Label          string
	DefaultEnabled bool
	Channel        string
	Topic          *string // NULL means "no topic" and "*" means "all topics"
	Event          *string // NULL means "no event" and "*" means "all events"
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

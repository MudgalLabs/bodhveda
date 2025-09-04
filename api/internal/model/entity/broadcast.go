package entity

import (
	"encoding/json"
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/enum"
)

type Broadcast struct {
	ID          int
	ProjectID   int
	Payload     json.RawMessage
	Channel     string
	Topic       string
	Event       string
	Status      enum.BroadcastStatus
	CompletedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewBroadcast(projectID int, payload json.RawMessage, channel string, topic string, event string) *Broadcast {
	now := time.Now().UTC()
	return &Broadcast{
		ProjectID:   projectID,
		Payload:     payload,
		Channel:     channel,
		Topic:       topic,
		Event:       event,
		Status:      enum.BroadcastStatusEnqueued,
		CompletedAt: nil,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

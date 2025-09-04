package entity

import (
	"encoding/json"
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/enum"
)

type Notification struct {
	ID             int
	ProjectID      int
	RecipientExtID string
	Payload        json.RawMessage
	BroadcastID    *int
	Channel        string
	Topic          string
	Event          string
	ReadAt         *time.Time
	OpenedAt       *time.Time
	Status         enum.NotificationStatus
	CompletedAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewNotification(projectID int, recipientExtID string, payload json.RawMessage, broadcastID *int, channel, topic, event string) *Notification {
	now := time.Now().UTC()

	return &Notification{
		ProjectID:      projectID,
		RecipientExtID: recipientExtID,
		Payload:        payload,
		BroadcastID:    broadcastID,
		Channel:        channel,
		Topic:          topic,
		Event:          event,
		ReadAt:         nil,
		OpenedAt:       nil,
		Status:         enum.NotificationStatusEnqueued,
		CompletedAt:    nil,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

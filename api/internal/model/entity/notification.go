package entity

import (
	"encoding/json"
	"time"
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
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewNotification(projectID int, recipientExtID string, payload json.RawMessage, broadcastID *int, channel string, topic string, event string) *Notification {
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
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

type PrepareBroadcastBatchesPayload struct {
	Broadcast *Broadcast
}

func NewPrepareBroadcastBatchesPayload(broadcast *Broadcast) *PrepareBroadcastBatchesPayload {
	return &PrepareBroadcastBatchesPayload{
		Broadcast: broadcast,
	}
}

type BroadcastDeliveryTaskPayload struct {
	ProjectID       int
	BroadcastID     int
	BatchID         int
	RecipientExtIDs []string
	Payload         json.RawMessage
	Channel         string
	Topic           string
	Event           string
}

func NewBroadcastDeliveryTaskPayload(projectID int, broadcastID int, batchID int, recipientExtIDs []string, payload json.RawMessage, channel string, topic string, event string) *BroadcastDeliveryTaskPayload {
	return &BroadcastDeliveryTaskPayload{
		ProjectID:       projectID,
		BroadcastID:     broadcastID,
		BatchID:         batchID,
		RecipientExtIDs: recipientExtIDs,
		Payload:         payload,
		Channel:         channel,
		Topic:           topic,
		Event:           event,
	}
}

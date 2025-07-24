// Package notification provides the core functionality for handling notifications in the system.
package notification

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	ID          uuid.UUID       `json:"id" db:"id"`
	ProjectID   uuid.UUID       `json:"project_id" db:"project_id"`
	Recipient   string          `json:"recipient" db:"recipient"`
	BroadcastID *uuid.UUID      `json:"broadcast_id" db:"broadcast_id"`
	Payload     json.RawMessage `json:"payload" db:"payload"`
	ReadAt      *time.Time      `json:"read_at" db:"read_at"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	ExpiresAt   time.Time       `json:"expires_at" db:"expires_at"`
}

func new(projectID uuid.UUID, recipient string, payload json.RawMessage, expiresAt time.Time) (*Notification, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	return &Notification{
		ID:        id,
		ProjectID: projectID,
		Recipient: recipient,
		Payload:   payload,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
	}, nil
}

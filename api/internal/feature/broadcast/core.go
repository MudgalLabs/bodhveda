// Package broadcast package provides functionality to create and manage broadcast notifications.
package broadcast

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Broadcast struct {
	ID        uuid.UUID       `json:"id" db:"id"`
	ProjectID uuid.UUID       `json:"project_id" db:"project_id"`
	Payload   json.RawMessage `json:"payload" db:"payload"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
	ExpiresAt time.Time       `json:"expires_at" db:"expires_at"`
}

func New(projectID uuid.UUID, payload json.RawMessage, expiresAt time.Time) (*Broadcast, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	return &Broadcast{
		ID:        id,
		ProjectID: projectID,
		Payload:   payload,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
	}, nil
}

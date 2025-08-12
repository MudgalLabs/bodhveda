package dto

import (
	"encoding/json"
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
)

type Broadcast struct {
	ID          int             `json:"id"`
	Payload     json.RawMessage `json:"payload"`
	Target      Target          `json:"target"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

func FromBroadcast(broadcast *entity.Broadcast) *Broadcast {
	if broadcast == nil {
		return nil
	}

	return &Broadcast{
		ID:      broadcast.ID,
		Payload: broadcast.Payload,
		Target: Target{
			Channel: broadcast.Channel,
			Topic:   broadcast.Topic,
			Event:   broadcast.Event,
		},
		CompletedAt: broadcast.CompletedAt,
		CreatedAt:   broadcast.CreatedAt,
		UpdatedAt:   broadcast.UpdatedAt,
	}
}

package dto

import (
	"encoding/json"
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/tantra/query"
)

type Broadcast struct {
	ID          int                  `json:"id"`
	Payload     json.RawMessage      `json:"payload"`
	Target      Target               `json:"target"`
	Status      enum.BroadcastStatus `json:"status"`
	CompletedAt *time.Time           `json:"completed_at,omitempty"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
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
		Status:      broadcast.Status,
		CompletedAt: broadcast.CompletedAt,
		CreatedAt:   broadcast.CreatedAt,
		UpdatedAt:   broadcast.UpdatedAt,
	}
}

type BroadcastListItem struct {
	Broadcast
	DeliveredCount int `json:"delivered_count"`
	ReadCount      int `json:"read_count"`
	OpenedCount    int `json:"opened_count"`
}

type ListBroadcastsFilters struct {
	ProjectID int

	query.Pagination
}

type ListBroadcastssResult struct {
	Broadcasts []*BroadcastListItem `json:"broadcasts"`
	Pagination query.PaginationMeta `json:"pagination"`
}

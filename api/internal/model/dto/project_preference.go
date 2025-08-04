package dto

import (
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/tantra/apires"
	"github.com/mudgallabs/tantra/service"
)

type ProjectPreference struct {
	ID             int       `json:"id"`
	Label          string    `json:"label"`
	DefaultEnabled bool      `json:"default_enabled"`
	Channel        string    `json:"channel"`
	Topic          *string   `json:"topic"`
	Event          *string   `json:"event"`
	CreatedAt      time.Time `json:"created_at"`
}

type CreateProjectPreferencePayload struct {
	ProjectID int

	Label          string  `json:"label"`
	DefaultEnabled bool    `json:"default_enabled"`
	Channel        string  `json:"channel"`
	Topic          *string `json:"topic"`
	Event          *string `json:"event"`
}

func (p *CreateProjectPreferencePayload) Validate() error {
	var errs service.InputValidationErrors

	if p.ProjectID <= 0 {
		errs.Add(apires.NewApiError("Project is required", "Project ID must be a positive integer", "project_id", p.ProjectID))
	}

	if p.Channel == "" {
		errs.Add(apires.NewApiError("Channel is required", "Channel cannot be empty", "channel", p.Channel))
	}

	if p.Event != nil && *p.Event == "" {
		errs.Add(apires.NewApiError("Event is required", "Event should either be null otherwise cannot be empty", "event", p.Event))
	}

	if p.Topic != nil && *p.Topic == "" {
		errs.Add(apires.NewApiError("Topic is required", "Topic should either be null otherwise cannot be empty", "topic", p.Topic))
	}

	if p.Label == "" {
		errs.Add(apires.NewApiError("Label is required", "Label cannot be empty", "label", p.Label))
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func FromProjectPreference(e *entity.ProjectPreference) *ProjectPreference {
	if e == nil {
		return nil
	}

	return &ProjectPreference{
		ID:             e.ID,
		Channel:        e.Channel,
		Topic:          e.Topic,
		Event:          e.Event,
		Label:          e.Label,
		DefaultEnabled: e.DefaultEnabled,
		CreatedAt:      e.CreatedAt,
	}
}

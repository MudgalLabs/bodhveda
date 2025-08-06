package dto

import (
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/tantra/apires"
	"github.com/mudgallabs/tantra/service"
)

type ProjectPreference struct {
	ID        int       `json:"id"`
	ProjectID int       `json:"project_id"`
	Channel   string    `json:"channel"`
	Topic     string    `json:"topic"`
	Event     string    `json:"event"`
	Enabled   bool      `json:"default_enabled"`
	Label     string    `json:"label"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateProjectPreferencePayload struct {
	ProjectID int

	Channel string `json:"channel"`
	Topic   string `json:"topic"`
	Event   string `json:"event"`
	Label   string `json:"label"`
	Enabled bool   `json:"default_enabled"`
}

func (p *CreateProjectPreferencePayload) Validate() error {
	var errs service.InputValidationErrors

	if p.ProjectID <= 0 {
		errs.Add(apires.NewApiError("Project is required", "Project ID must be a positive integer", "project_id", p.ProjectID))
	}

	if p.Channel == "" {
		errs.Add(apires.NewApiError("Channel is required", "Channel cannot be empty", "channel", p.Channel))
	}

	if p.Event == "" {
		errs.Add(apires.NewApiError("Event is required", "Event cannot be empty", "event", p.Event))
	}

	if p.Topic == "" {
		errs.Add(apires.NewApiError("Topic is required", "Topic cannot be empty", "topic", p.Topic))
	}

	if p.Label == "" {
		errs.Add(apires.NewApiError("Label is required", "Label cannot be empty", "label", p.Label))
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func FromPreferenceForProject(e *entity.Preference) *ProjectPreference {
	if e == nil {
		return nil
	}

	return &ProjectPreference{
		ID:        e.ID,
		ProjectID: *e.ProjectID,
		Channel:   e.Channel,
		Topic:     e.Topic,
		Event:     e.Event,
		Enabled:   e.Enabled,
		Label:     *e.Label,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}
}

type RecipientPreference struct {
	ID             int       `json:"id"`
	RecipientExtID string    `json:"recipient_id"`
	Channel        string    `json:"channel"`
	Topic          string    `json:"topic"`
	Event          string    `json:"event"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type UpsertRecipientPreferencePayload struct {
	ProjectID      int
	RecipientExtID string

	Channel string `json:"channel"`
	Topic   string `json:"topic"`
	Event   string `json:"event"`
	Enabled bool   `json:"enabled"`
}

func (p *UpsertRecipientPreferencePayload) Validate() error {
	var errs service.InputValidationErrors

	if p.RecipientExtID == "" {
		errs.Add(apires.NewApiError("Recipient is required", "Recipient ID cannot be empty", "recipient_id", p.RecipientExtID))
	}

	if p.Channel == "" {
		errs.Add(apires.NewApiError("Channel is required", "Channel cannot be empty", "channel", p.Channel))
	}

	if p.Event == "" {
		errs.Add(apires.NewApiError("Event is required", "Event cannot be empty", "event", p.Event))
	}

	if p.Topic == "" {
		errs.Add(apires.NewApiError("Topic is required", "Topic cannot be empty", "topic", p.Topic))
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func FromPreferenceForRecipient(e *entity.Preference) *RecipientPreference {
	if e == nil {
		return nil
	}

	recipientExtID := ""
	if e.RecipientExtID != nil {
		recipientExtID = *e.RecipientExtID
	}

	return &RecipientPreference{
		ID:             e.ID,
		RecipientExtID: recipientExtID,
		Channel:        e.Channel,
		Topic:          e.Topic,
		Event:          e.Event,
		Enabled:        e.Enabled,
		CreatedAt:      e.CreatedAt,
		UpdatedAt:      e.UpdatedAt,
	}
}

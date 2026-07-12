package dto

import (
	"strings"
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/tantra/apires"
	"github.com/mudgallabs/tantra/service"
)

// normalizeMedium trims/lowercases a request-supplied medium and falls back to
// the default (in_app) when omitted, keeping the preference API backward
// compatible for callers that predate mediums.
func normalizeMedium(m string) enum.Medium {
	m = strings.ToLower(strings.TrimSpace(m))
	if m == "" {
		return enum.DefaultMedium
	}
	return enum.Medium(m)
}

// validateMedium reports whether m is an active preference medium (in_app or
// email in v1). When invalid, ok is false and the returned ApiError describes it.
func validateMedium(m enum.Medium) (apires.ApiError, bool) {
	if !m.Active() {
		return apires.NewApiError("Invalid medium", "Medium must be one of: in_app, email", "medium", string(m)), false
	}
	return apires.ApiError{}, true
}

type ProjectPreference struct {
	ID        int       `json:"id"`
	ProjectID int       `json:"project_id"`
	Target    Target    `json:"target"`
	Medium    string    `json:"medium"`
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
	// Medium defaults to in_app when omitted. A project preference is a catalog
	// entry: it declares that this (target, medium) may fire.
	Medium  string `json:"medium"`
	Label   string `json:"label"`
	Enabled bool   `json:"default_enabled"`
}

func (p *CreateProjectPreferencePayload) Validate() error {
	var errs service.InputValidationErrors

	if p.ProjectID <= 0 {
		errs.Add(apires.NewApiError("Project is required", "Project ID must be a positive integer", "project_id", p.ProjectID))
	}

	p.Medium = string(normalizeMedium(p.Medium))
	if apiErr, ok := validateMedium(enum.Medium(p.Medium)); !ok {
		errs.Add(apiErr)
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
		Target: Target{
			Channel: e.Channel,
			Topic:   e.Topic,
			Event:   e.Event,
		},
		Medium:    e.Medium,
		Enabled:   e.Enabled,
		Label:     *e.Label,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}
}

type RecipientPreference struct {
	ID             int       `json:"id"`
	ProjectID      int       `json:"project_id"`
	RecipientExtID string    `json:"recipient_id"`
	Target         Target    `json:"target"`
	Medium         string    `json:"medium"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type UpsertRecipientPreferencePayload struct {
	ProjectID int

	RecipientExtID string `json:"recipient_id"`
	Channel        string `json:"channel"`
	Topic          string `json:"topic"`
	Event          string `json:"event"`
	// Medium defaults to in_app when omitted.
	Medium  string `json:"medium"`
	Enabled bool   `json:"enabled"`
}

func (p *UpsertRecipientPreferencePayload) Validate() error {
	var errs service.InputValidationErrors

	if p.ProjectID <= 0 {
		errs.Add(apires.NewApiError("Project is required", "Project ID must be a positive integer", "project_id", p.ProjectID))
	}

	p.Medium = string(normalizeMedium(p.Medium))
	if apiErr, ok := validateMedium(enum.Medium(p.Medium)); !ok {
		errs.Add(apiErr)
	}

	if p.RecipientExtID == "" {
		errs.Add(apires.NewApiError("Recipient is required", "Recipient ID cannot be empty", "recipient_id", p.RecipientExtID))
	} else {
		p.RecipientExtID = strings.ToLower(p.RecipientExtID)
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

	return &RecipientPreference{
		ID:             e.ID,
		ProjectID:      *e.ProjectID,
		RecipientExtID: *e.RecipientExtID,
		Target: Target{
			Channel: e.Channel,
			Topic:   e.Topic,
			Event:   e.Event,
		},
		Medium:    e.Medium,
		Enabled:   e.Enabled,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}
}

type ProjectPreferenceListItem struct {
	ProjectPreference

	Subscribers int `json:"subscribers"`
}

func FromProjectPreferenceList(list []*entity.ProjectPreferenceListItem) []*ProjectPreferenceListItem {
	if list == nil {
		return nil
	}

	DTOs := make([]*ProjectPreferenceListItem, len(list))
	for i, item := range list {
		projectPreferenctDTO := FromPreferenceForProject(&item.Preference)
		proejctPreferenceListItem := &ProjectPreferenceListItem{
			ProjectPreference: *projectPreferenctDTO,
			Subscribers:       item.Subscribers,
		}
		DTOs[i] = proejctPreferenceListItem
	}

	return DTOs
}

type PreferenceTarget struct {
	Target
	Medium string  `json:"medium"`
	Label  *string `json:"label,omitempty"`
}

type PreferenceState struct {
	Enabled   bool `json:"enabled"`
	Inherited bool `json:"inherited"`
}

type PreferenceTargetStateDTO struct {
	Target PreferenceTarget `json:"target"`
	State  PreferenceState  `json:"state"`
}

type PreferenceTargetStatesResultDTO struct {
	Preferences []*PreferenceTargetStateDTO `json:"preferences"`
}

type PatchRecipientPreferenceTargetPayload struct {
	Target PreferenceTarget `json:"target"`
	// Medium defaults to in_app when omitted. It sits alongside target so the
	// recipient can toggle in-app and email for the same target independently.
	Medium string `json:"medium"`
	State  struct {
		Enabled bool `json:"enabled"`
	} `json:"state"`
}

func (p *PatchRecipientPreferenceTargetPayload) Validate() error {
	var errs service.InputValidationErrors

	p.Medium = string(normalizeMedium(p.Medium))
	if apiErr, ok := validateMedium(enum.Medium(p.Medium)); !ok {
		errs.Add(apiErr)
	}

	if p.Target.Channel == "" {
		errs.Add(apires.NewApiError("Channel is required", "Channel cannot be empty", "channel", p.Target.Channel))
	}
	if p.Target.Topic == "" {
		errs.Add(apires.NewApiError("Topic is required", "Topic cannot be empty", "topic", p.Target.Topic))
	}
	if p.Target.Event == "" {
		errs.Add(apires.NewApiError("Event is required", "Event cannot be empty", "event", p.Target.Event))
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

type PatchRecipientPreferenceTargetResult = PreferenceTargetStateDTO

func PreferenceTargetDTOFromPreference(e *entity.Preference) PreferenceTarget {
	return PreferenceTarget{
		Target: Target{
			Channel: e.Channel,
			Topic:   e.Topic,
			Event:   e.Event,
		},
		Medium: e.Medium,
		Label:  e.Label,
	}
}

func PreferenceTargetStateDTOFromPreference(e *entity.Preference, inherited bool) *PreferenceTargetStateDTO {
	return &PreferenceTargetStateDTO{
		Target: PreferenceTargetDTOFromPreference(e),
		State: PreferenceState{
			Enabled:   e.Enabled,
			Inherited: inherited,
		},
	}
}

type CheckRecipientTargetPayload struct {
	Target
	// Medium defaults to in_app when omitted (query param `medium`).
	Medium string `json:"medium" schema:"medium"`
}

func (q *CheckRecipientTargetPayload) Validate() error {
	var errs service.InputValidationErrors

	q.Medium = string(normalizeMedium(q.Medium))
	if apiErr, ok := validateMedium(enum.Medium(q.Medium)); !ok {
		errs.Add(apiErr)
	}

	if q.Channel == "" {
		errs.Add(apires.NewApiError("Channel is required", "Channel cannot be empty", "channel", q.Channel))
	}
	if q.Topic == "" {
		errs.Add(apires.NewApiError("Topic is required", "Topic cannot be empty", "topic", q.Topic))
	}
	if q.Event == "" {
		errs.Add(apires.NewApiError("Event is required", "Event cannot be empty", "event", q.Event))
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

type DeletePreferencePayload struct {
	ProjectID    int `json:"project_id"`
	PreferenceID int `json:"preference_id"`
}

func (p *DeletePreferencePayload) Validate() error {
	var errs service.InputValidationErrors

	if p.ProjectID <= 0 {
		errs.Add(apires.NewApiError("Project is required", "Project ID must be a positive integer", "project_id", p.ProjectID))
	}

	if p.PreferenceID <= 0 {
		errs.Add(apires.NewApiError("Preference ID is required", "Preference ID must be a positive integer", "preference_id", p.PreferenceID))
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

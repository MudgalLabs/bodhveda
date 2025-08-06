package dto

import (
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/tantra/apires"
	"github.com/mudgallabs/tantra/service"
)

type Recipient struct {
	ExternalID string    `json:"recipient_id"` // Unique recipient ID from the client's system.
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type CreateRecipientPayload struct {
	ProjectID int

	ExternalID string  `json:"recipient_id"`
	Name       *string `json:"name"`
}

func (p *CreateRecipientPayload) Validate() error {
	var errs service.InputValidationErrors

	if p.ProjectID <= 0 {
		errs.Add(apires.NewApiError("Project is required", "Project ID must be a positive integer", "project_id", p.ProjectID))
	}

	if p.ExternalID == "" {
		errs.Add(apires.NewApiError("Recipient ID is required", "Recipient ID cannot be empty", "recipient_id", p.ExternalID))
	}

	if p.Name != nil && *p.Name == "" {
		errs.Add(apires.NewApiError("Name is required", "Name cannot be empty", "name", p.Name))
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func FromRecipient(r *entity.Recipient) *Recipient {
	if r == nil {
		return nil
	}

	return &Recipient{
		ExternalID: r.ExternalID,
		Name:       r.Name,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
}

type RecipientListItem struct {
	Recipient

	DirectNotificationsCount    int `json:"direct_notifications_count"`
	BroadcastNotificationsCount int `json:"broadcast_notifications_count"`
}

func FromRecipientList(r []*entity.RecipientListItem) []*RecipientListItem {
	if r == nil {
		return nil
	}

	dtos := make([]*RecipientListItem, len(r))
	for i, recipient := range r {
		recipientDto := FromRecipient(&recipient.Recipient)
		recipientListItem := &RecipientListItem{
			Recipient:                   *recipientDto,
			DirectNotificationsCount:    recipient.DirectNotificationsCount,
			BroadcastNotificationsCount: recipient.BroadcastNotificationsCount,
		}
		dtos[i] = recipientListItem
	}

	return dtos
}

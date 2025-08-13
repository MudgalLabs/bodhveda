package dto

import (
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/tantra/apires"
	"github.com/mudgallabs/tantra/query"
	"github.com/mudgallabs/tantra/service"
)

type Recipient struct {
	ExternalID string    `json:"id"` // Unique recipient ID from the client's system.
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type CreateRecipientPayload struct {
	ProjectID int

	ExternalID string  `json:"id"`
	Name       *string `json:"name"`
}

func (p *CreateRecipientPayload) Validate() error {
	var errs service.InputValidationErrors

	if p.ProjectID <= 0 {
		errs.Add(apires.NewApiError("Project is required", "Project ID must be a positive integer", "project_id", p.ProjectID))
	}

	if p.ExternalID == "" {
		errs.Add(apires.NewApiError("ID is required", "ID cannot be empty", "id", p.ExternalID))
	}

	if p.Name != nil && *p.Name == "" {
		errs.Add(apires.NewApiError("Name is required", "Name cannot be empty", "name", p.Name))
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

type UpdateRecipientPayload struct {
	Name *string `json:"name"`
}

func (p *UpdateRecipientPayload) Validate() error {
	var errs service.InputValidationErrors

	if p.Name != nil && *p.Name == "" {
		errs.Add(apires.NewApiError("Name is required", "Name cannot be empty", "name", p.Name))
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

type BatchCreateRecipientsPayload struct {
	Recipients []CreateRecipientPayload `json:"recipients"`
}

type BatchCreateRecipientCreated struct {
	RecipientExtID string `json:"id"`
}

type BatchCreateRecipientUpdated struct {
	RecipientExtID string `json:"id"`
}

type BatchCreateRecipientFailed struct {
	Errors         service.InputValidationErrors `json:"errors"`
	RecipientExtID string                        `json:"id"`
	BatchIndex     int                           `json:"batch_index"`
}

type BatchCreateRecipientsResult struct {
	Created []BatchCreateRecipientCreated `json:"created"`
	Updated []BatchCreateRecipientUpdated `json:"updated"`
	Failed  []BatchCreateRecipientFailed  `json:"failed"`
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

func FromRecipients(recipient []*entity.Recipient) []*Recipient {
	if recipient == nil {
		return nil
	}

	list := make([]*Recipient, len(recipient))
	for i, r := range recipient {
		list[i] = FromRecipient(r)
	}

	return list
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

	DTOs := make([]*RecipientListItem, len(r))
	for i, recipient := range r {
		recipientDTO := FromRecipient(&recipient.Recipient)
		recipientListItem := &RecipientListItem{
			Recipient:                   *recipientDTO,
			DirectNotificationsCount:    recipient.DirectNotificationsCount,
			BroadcastNotificationsCount: recipient.BroadcastNotificationsCount,
		}
		DTOs[i] = recipientListItem
	}

	return DTOs
}

type DeleteRecipientDataPayload struct {
	ProjectID      int    `json:"project_id"`
	RecipientExtID string `json:"recipient_ext_id"`
}

type ListRecipientsPayload struct {
	query.Pagination
	ProjectID int `json:"project_id"`
}

type ListRecipientsResult struct {
	Recipients []*RecipientListItem `json:"recipients"`
	Pagination query.PaginationMeta `json:"pagination"`
}

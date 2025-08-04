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

func FromRecipients(r []*entity.Recipient) []*Recipient {
	if r == nil {
		return nil
	}

	dtos := make([]*Recipient, len(r))
	for i, recipient := range r {
		dtos[i] = FromRecipient(recipient)
	}

	return dtos
}

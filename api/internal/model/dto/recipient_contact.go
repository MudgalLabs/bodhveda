package dto

import (
	"strings"
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/tantra/apires"
	"github.com/mudgallabs/tantra/service"
)

// RecipientContact is the API representation of a recipient's contact address.
type RecipientContact struct {
	ID         int64      `json:"id"`
	Medium     string     `json:"medium"`
	Address    string     `json:"address"`
	IsPrimary  bool       `json:"is_primary"`
	VerifiedAt *time.Time `json:"verified_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func FromRecipientContact(c *entity.RecipientContact) *RecipientContact {
	if c == nil {
		return nil
	}

	return &RecipientContact{
		ID:         c.ID,
		Medium:     string(c.Medium),
		Address:    c.Address,
		IsPrimary:  c.IsPrimary,
		VerifiedAt: c.VerifiedAt,
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
	}
}

func FromRecipientContacts(contacts []*entity.RecipientContact) []*RecipientContact {
	list := make([]*RecipientContact, len(contacts))
	for i, c := range contacts {
		list[i] = FromRecipientContact(c)
	}
	return list
}

// normalizeAddress trims addresses and lowercases email (case-insensitive), while
// leaving other mediums' addresses (e.g. push tokens) byte-for-byte intact.
func normalizeAddress(medium enum.Medium, address string) string {
	address = strings.TrimSpace(address)
	if medium == enum.MediumEmail {
		address = strings.ToLower(address)
	}
	return address
}

type CreateRecipientContactPayload struct {
	ProjectID      int
	RecipientExtID string

	Medium    string `json:"medium"`
	Address   string `json:"address"`
	IsPrimary bool   `json:"is_primary"`
}

func (p *CreateRecipientContactPayload) Validate() error {
	var errs service.InputValidationErrors

	if p.ProjectID <= 0 {
		errs.Add(apires.NewApiError("Project is required", "Project ID must be a positive integer", "project_id", p.ProjectID))
	}

	if p.RecipientExtID == "" {
		errs.Add(apires.NewApiError("Recipient is required", "Recipient ID cannot be empty", "recipient_id", p.RecipientExtID))
	}

	medium := enum.Medium(strings.TrimSpace(p.Medium))
	if !medium.ValidContactMedium() {
		errs.Add(apires.NewApiError("Invalid medium", "Medium must be one of: email, sms, web_push, mobile_push", "medium", p.Medium))
	} else {
		p.Medium = string(medium)
	}

	p.Address = normalizeAddress(medium, p.Address)
	if p.Address == "" {
		errs.Add(apires.NewApiError("Address is required", "Address cannot be empty", "address", p.Address))
	} else if medium == enum.MediumEmail && !strings.Contains(p.Address, "@") {
		errs.Add(apires.NewApiError("Invalid email address", "Email address must contain '@'", "address", p.Address))
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

// UpdateRecipientContactPayload updates a contact. Both fields are optional; only
// the provided fields are applied. Changing the address invalidates verification.
type UpdateRecipientContactPayload struct {
	ProjectID      int
	RecipientExtID string
	ContactID      int64

	// Medium is the existing contact's medium, injected by the service after the
	// contact is loaded, so the address can be normalized correctly. Not part of
	// the request body.
	Medium enum.Medium `json:"-"`

	Address   *string `json:"address"`
	IsPrimary *bool   `json:"is_primary"`
}

func (p *UpdateRecipientContactPayload) Validate() error {
	var errs service.InputValidationErrors

	if p.Address == nil && p.IsPrimary == nil {
		errs.Add(apires.NewApiError("Nothing to update", "Provide at least one of: address, is_primary", "address", nil))
	}

	if p.Address != nil {
		normalized := normalizeAddress(p.Medium, *p.Address)
		if normalized == "" {
			errs.Add(apires.NewApiError("Address is required", "Address cannot be empty", "address", *p.Address))
		} else if p.Medium == enum.MediumEmail && !strings.Contains(normalized, "@") {
			errs.Add(apires.NewApiError("Invalid email address", "Email address must contain '@'", "address", *p.Address))
		}
		p.Address = &normalized
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

// SetPrimaryContactPayload is the body of the idempotent "ensure this is the
// primary contact for this medium" upsert (PUT). Unlike CreateRecipientContactPayload
// there is no is_primary field — setting the primary IS the operation.
type SetPrimaryContactPayload struct {
	ProjectID      int
	RecipientExtID string

	Medium  string `json:"medium"`
	Address string `json:"address"`
}

func (p *SetPrimaryContactPayload) Validate() error {
	var errs service.InputValidationErrors

	if p.ProjectID <= 0 {
		errs.Add(apires.NewApiError("Project is required", "Project ID must be a positive integer", "project_id", p.ProjectID))
	}

	if p.RecipientExtID == "" {
		errs.Add(apires.NewApiError("Recipient is required", "Recipient ID cannot be empty", "recipient_id", p.RecipientExtID))
	}

	medium := enum.Medium(strings.TrimSpace(p.Medium))
	if !medium.ValidContactMedium() {
		errs.Add(apires.NewApiError("Invalid medium", "Medium must be one of: email, sms, web_push, mobile_push", "medium", p.Medium))
	} else {
		p.Medium = string(medium)
	}

	p.Address = normalizeAddress(medium, p.Address)
	if p.Address == "" {
		errs.Add(apires.NewApiError("Address is required", "Address cannot be empty", "address", p.Address))
	} else if medium == enum.MediumEmail && !strings.Contains(p.Address, "@") {
		errs.Add(apires.NewApiError("Invalid email address", "Email address must contain '@'", "address", p.Address))
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

type ListRecipientContactsResult struct {
	Contacts []*RecipientContact `json:"contacts"`
}

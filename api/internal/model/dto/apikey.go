package dto

import (
	"time"

	"github.com/mudgallabs/bodhveda/internal/env"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/tantra/apires"
	"github.com/mudgallabs/tantra/cipher"
	"github.com/mudgallabs/tantra/logger"
	"github.com/mudgallabs/tantra/service"
)

type APIKey struct {
	ID          int              `json:"id"`
	Name        string           `json:"name"`
	TokenParial string           `json:"token_partial"`
	Scope       enum.APIKeyScope `json:"scope"`
	CreatedAt   time.Time        `json:"created_at"`
}

type CreateAPIKeyPayload struct {
	UserID    int
	ProjectID int

	Name  string           `json:"name"`
	Scope enum.APIKeyScope `json:"scope"`
}

func (p *CreateAPIKeyPayload) Validate() error {
	var errs service.InputValidationErrors

	if p.UserID <= 0 {
		errs.Add(apires.NewApiError("User is required", "User ID must be a positive integer", "user_id", p.UserID))
	}

	if p.ProjectID <= 0 {
		errs.Add(apires.NewApiError("Project is required", "Project ID must be a positive integer", "project_id", p.ProjectID))
	}

	if p.Name == "" {
		errs.Add(apires.NewApiError("Name is required", "Name cannot be empty", "name", p.Name))
	}

	if p.Scope != enum.APIKeyScopeFull && p.Scope != enum.APIKeyScopeRecipient {
		errs.Add(apires.NewApiError("Invalid scope", "Scope must be either 'all' or 'recipient'", "scope", p.Scope))
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func FromAPIKey(a *entity.APIKey) *APIKey {
	if a == nil {
		return nil
	}

	tokenPlain, err := cipher.Decrypt(a.Token, a.Nonce, []byte(env.CipherKey))
	if err != nil {
		logger.Get().DPanicw("Failed to decrypt API key token", "error", err)
	}

	return &APIKey{
		ID:          a.ID,
		Name:        a.Name,
		TokenParial: tokenPlain[:12] + "...", // Return first 8 characters of the token
		Scope:       a.Scope,
		CreatedAt:   a.CreatedAt,
	}
}

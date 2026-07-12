package dto

import (
	"strings"
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/tantra/apires"
	"github.com/mudgallabs/tantra/service"
)

// ProjectEmailSettings is the API (console) representation of a project's email
// provider settings. It NEVER carries the plaintext secret — only a masked hint
// (SecretMasked) so the console can confirm which key is configured without
// exposing it.
type ProjectEmailSettings struct {
	Provider     string    `json:"provider"`
	FromName     string    `json:"from_name"`
	FromAddress  string    `json:"from_address"`
	SecretMasked string    `json:"secret_masked"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// MaskSecret turns a plaintext provider secret into a display-safe hint that
// reveals only the last 4 characters (e.g. "••••••••wxyz"). Short/empty secrets
// are fully masked.
func MaskSecret(plain string) string {
	const dots = "••••••••"
	if len(plain) <= 4 {
		return dots
	}
	return dots + plain[len(plain)-4:]
}

// UpsertProjectEmailSettingsPayload sets or rotates a project's email settings.
//
// Secret carries the provider API key in plaintext on the way IN only. It is
// optional on update: when a config already exists and Secret is blank, the
// existing encrypted secret is kept (identity/provider-only update); when no
// config exists yet, Secret is required. Provider defaults to Resend.
type UpsertProjectEmailSettingsPayload struct {
	ProjectID int

	Provider    string `json:"provider"`
	Secret      string `json:"secret"`
	FromName    string `json:"from_name"`
	FromAddress string `json:"from_address"`

	// hasExisting is set by the service before Validate so a rotation can omit
	// the secret only when there is an existing one to keep.
	hasExisting bool `json:"-"`
}

// SetHasExisting records whether a config already exists, so Validate can require
// the secret only on first configuration.
func (p *UpsertProjectEmailSettingsPayload) SetHasExisting(v bool) {
	p.hasExisting = v
}

func (p *UpsertProjectEmailSettingsPayload) Validate() error {
	var errs service.InputValidationErrors

	if p.ProjectID <= 0 {
		errs.Add(apires.NewApiError("Project is required", "Project ID must be a positive integer", "project_id", p.ProjectID))
	}

	// Provider is optional; default to Resend. Reject anything else.
	if strings.TrimSpace(p.Provider) == "" {
		p.Provider = string(enum.DefaultEmailProvider)
	} else {
		p.Provider = strings.TrimSpace(p.Provider)
		if !enum.EmailProvider(p.Provider).Valid() {
			errs.Add(apires.NewApiError("Invalid provider", "Provider must be one of: resend", "provider", p.Provider))
		}
	}

	// Secret is required only when configuring for the first time. On a subsequent
	// update the caller may omit it to keep the existing key (identity-only edit).
	p.Secret = strings.TrimSpace(p.Secret)
	if p.Secret == "" && !p.hasExisting {
		errs.Add(apires.NewApiError("Secret is required", "Provide the provider API key", "secret", ""))
	}

	p.FromName = strings.TrimSpace(p.FromName)
	if p.FromName == "" {
		errs.Add(apires.NewApiError("From name is required", "From name cannot be empty", "from_name", p.FromName))
	}

	p.FromAddress = strings.ToLower(strings.TrimSpace(p.FromAddress))
	if p.FromAddress == "" {
		errs.Add(apires.NewApiError("From address is required", "From address cannot be empty", "from_address", p.FromAddress))
	} else if !strings.Contains(p.FromAddress, "@") {
		errs.Add(apires.NewApiError("Invalid from address", "From address must be a valid email", "from_address", p.FromAddress))
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

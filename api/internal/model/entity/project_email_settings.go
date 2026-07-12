package entity

import (
	"fmt"
	"time"

	"github.com/mudgallabs/bodhveda/internal/env"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/tantra/cipher"
)

// ProjectEmailSettings is a project's BYO email provider configuration: the
// provider discriminator, the provider secret (Resend API key) encrypted at rest
// exactly like an api_key token (Secret = AES-GCM ciphertext, Nonce), and the
// "from" identity outbound email is sent as.
//
// Secret is never serialized to clients in plaintext — see dto.ProjectEmailSettings,
// which only exposes a masked hint. Use SetSecret to (re)encrypt a new plaintext
// secret when rotating.
type ProjectEmailSettings struct {
	ProjectID   int
	Provider    enum.EmailProvider
	Secret      []byte // Encrypted provider secret (Resend API key).
	Nonce       []byte // Nonce used for encryption.
	FromName    string
	FromAddress string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewProjectEmailSettings builds settings with a freshly encrypted secret.
func NewProjectEmailSettings(projectID int, provider enum.EmailProvider, plainSecret, fromName, fromAddress string) (*ProjectEmailSettings, error) {
	now := time.Now().UTC()

	s := &ProjectEmailSettings{
		ProjectID:   projectID,
		Provider:    provider,
		FromName:    fromName,
		FromAddress: fromAddress,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.SetSecret(plainSecret); err != nil {
		return nil, err
	}

	return s, nil
}

// SetSecret encrypts plainSecret and stores it as Secret + Nonce. The plaintext
// is not retained on the struct.
func (s *ProjectEmailSettings) SetSecret(plainSecret string) error {
	secret, nonce, err := cipher.Encrypt([]byte(plainSecret), []byte(env.CipherKey))
	if err != nil {
		return fmt.Errorf("encrypt provider secret: %w", err)
	}

	s.Secret = secret
	s.Nonce = nonce
	return nil
}

// DecryptSecret returns the plaintext provider secret. Callers that send email
// (Phase 4) use this; it must never be returned to a client. Masking for display
// is derived from this in the service layer.
func (s *ProjectEmailSettings) DecryptSecret() (string, error) {
	return cipher.Decrypt(s.Secret, s.Nonce, []byte(env.CipherKey))
}

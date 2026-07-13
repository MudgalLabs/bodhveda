// Package email holds the medium adapter interface and its provider
// implementations. An adapter turns a normalized outbound email into a
// provider-specific API call (v1: Resend, BYO-provider) and normalizes the
// result back into a provider message id we can correlate webhooks against
// (Phase 5). The provider is selected via the `enum.EmailProvider` discriminator
// stored on the project's email settings.
package email

import (
	"context"
	"fmt"

	"github.com/mudgallabs/bodhveda/internal/model/enum"
)

// Message is a normalized outbound email, provider-agnostic. FromName /
// FromAddress come from the project's email settings; the rest come from the
// send call's `email` block + the resolved recipient contact address.
type Message struct {
	FromName    string
	FromAddress string
	To          string
	Subject     string
	HTML        string
	Text        string
}

// SendResult is the normalized outcome of a provider send. ProviderMessageID is
// the id the provider assigns (used to match inbound webhooks in Phase 5).
type SendResult struct {
	Provider          enum.EmailProvider
	ProviderMessageID string
}

// Adapter sends a normalized Message via a specific provider.
type Adapter interface {
	// Provider reports which provider this adapter targets.
	Provider() enum.EmailProvider
	// Send dispatches the message. A non-nil error means the send failed (the
	// caller records a failed delivery); a nil error returns the provider id.
	Send(ctx context.Context, msg Message) (SendResult, error)
}

// NewAdapter builds the adapter for a provider, given the project's decrypted
// provider secret (API key). Only Resend is wired in v1; the discriminator makes
// adding Postmark/Mailgun/managed-SES a matter of adding a case here.
func NewAdapter(provider enum.EmailProvider, apiKey string) (Adapter, error) {
	switch provider {
	case enum.EmailProviderResend:
		return NewResendAdapter(apiKey), nil
	default:
		return nil, fmt.Errorf("unsupported email provider: %q", provider)
	}
}

package enum

// EmailProvider discriminates which email adapter a project's email settings
// target. Only Resend is wired in v1 (BYO-provider, Resend first); the type
// exists so more adapters (Postmark, Mailgun, a future managed SES tier) can be
// added without a schema change. Matches the `project_email_settings.provider`
// CHECK constraint.
type EmailProvider string

const (
	EmailProviderResend EmailProvider = "resend"
)

// DefaultEmailProvider is assumed when a request omits one.
const DefaultEmailProvider = EmailProviderResend

// Valid reports whether p is a known, accepted provider — i.e. a value the
// `project_email_settings.provider` CHECK constraint accepts.
func (p EmailProvider) Valid() bool {
	switch p {
	case EmailProviderResend:
		return true
	default:
		return false
	}
}

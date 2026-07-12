package enum

// Medium is a delivery transport for a notification.
//
// Phase 1 only introduces the transports that a recipient can carry a *contact
// address* for. The in-app inbox (`in_app`) is intentionally NOT a contact
// medium — its "address" is the recipient's external_id — so it is not listed
// here. Phase 2 introduces a broader shared medium concept (including `in_app`)
// on preferences; the values below are the contact-addressable subset that must
// stay in sync with the `recipient_contact.medium` CHECK constraint.
type Medium string

const (
	MediumEmail      Medium = "email"
	MediumSMS        Medium = "sms"
	MediumWebPush    Medium = "web_push"
	MediumMobilePush Medium = "mobile_push"
)

// ValidContactMedium reports whether m is a transport a recipient_contact can be
// stored for. Only `email` is exercised in v1, but the rest are accepted so the
// contacts table is future-proof without a re-migration when the next medium
// (web_push) lands.
func (m Medium) ValidContactMedium() bool {
	switch m {
	case MediumEmail, MediumSMS, MediumWebPush, MediumMobilePush:
		return true
	default:
		return false
	}
}

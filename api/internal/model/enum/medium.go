package enum

// Medium is a delivery transport for a notification.
//
// Two overlapping subsets matter:
//
//   - Contact-addressable mediums (email, sms, web_push, mobile_push) are the
//     transports a recipient_contact carries an *address* for. The in-app inbox
//     is NOT one — its "address" is the recipient's external_id. See
//     ValidContactMedium (introduced in Phase 1 for the contacts table).
//   - Preference/catalog mediums (all five below) are the transports a
//     preference row can gate. `in_app` is a first-class preference medium here;
//     legacy preference rows backfill to it. See Valid, which matches the
//     `preference.medium` CHECK constraint.
//
// Only `in_app` and `email` are *active* transports in v1 (Active); the rest are
// scaffolded so the enum, contacts table, and preference catalog don't need a
// re-migration when the next medium (web_push) lands.
type Medium string

const (
	MediumInApp      Medium = "in_app"
	MediumEmail      Medium = "email"
	MediumSMS        Medium = "sms"
	MediumWebPush    Medium = "web_push"
	MediumMobilePush Medium = "mobile_push"
)

// DefaultMedium is the medium assumed when a request omits one. It keeps the
// preference API backward compatible: a caller (or an older SDK) that doesn't
// send a medium gets in-app, exactly as before mediums existed.
const DefaultMedium = MediumInApp

// Valid reports whether m is any known medium — i.e. a value the
// `preference.medium` CHECK constraint accepts.
func (m Medium) Valid() bool {
	switch m {
	case MediumInApp, MediumEmail, MediumSMS, MediumWebPush, MediumMobilePush:
		return true
	default:
		return false
	}
}

// Active reports whether m is a transport that actually delivers in v1. Only
// in-app and email are wired; the others are scaffolded. Preference/catalog
// creation is restricted to active mediums so callers can't catalog a medium
// that can never fire.
func (m Medium) Active() bool {
	switch m {
	case MediumInApp, MediumEmail:
		return true
	default:
		return false
	}
}

// ValidContactMedium reports whether m is a transport a recipient_contact can be
// stored for. Only `email` is exercised in v1, but the rest are accepted so the
// contacts table is future-proof without a re-migration when the next medium
// (web_push) lands. `in_app` is intentionally excluded — it has no contact
// address.
func (m Medium) ValidContactMedium() bool {
	switch m {
	case MediumEmail, MediumSMS, MediumWebPush, MediumMobilePush:
		return true
	default:
		return false
	}
}

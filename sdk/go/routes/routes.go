// Package routes defines API endpoint routes for the Bodhveda.
package routes

import (
	"net/url"
	"strconv"
)

var (
	NotificationsSend = "/notifications/send"

	// Project preference CATALOG (project-scoped by the API key). Distinct from
	// the per-recipient preference routes below, which are one recipient's own
	// toggles. Upsert is the PUT variant of the same collection path.
	PreferencesList   = "/preferences"
	PreferencesCreate = "/preferences"
	PreferencesUpsert = "/preferences"
	PreferencesGet    = func(preferenceID int64) string { return "/preferences/" + strconv.FormatInt(preferenceID, 10) }
	PreferencesUpdate = func(preferenceID int64) string { return "/preferences/" + strconv.FormatInt(preferenceID, 10) }
	PreferencesDelete = func(preferenceID int64) string { return "/preferences/" + strconv.FormatInt(preferenceID, 10) }

	RecipientsCreate      = "/recipients"
	RecipientsCreateBatch = "/recipients/batch"
	RecipeientsGet        = func(recipientID string) string { return "/recipients/" + url.PathEscape(recipientID) }
	RecipeientsUpdate     = func(recipientID string) string { return "/recipients/" + url.PathEscape(recipientID) }
	RecipeientsDelete     = func(recipientID string) string { return "/recipients/" + url.PathEscape(recipientID) }

	RecipientsNotificationsList = func(recipientID string) string {
		return "/recipients/" + url.PathEscape(recipientID) + "/notifications"
	}
	RecipientsNotificationUnreadCount = func(recipientID string) string {
		return "/recipients/" + url.PathEscape(recipientID) + "/notifications/unread-count"
	}
	RecipientsNotificationsUpdateState = func(recipientID string) string {
		return "/recipients/" + url.PathEscape(recipientID) + "/notifications"
	}
	RecipientsNotificationsDelete = func(recipientID string) string {
		return "/recipients/" + url.PathEscape(recipientID) + "/notifications"
	}

	RecipientsPreferencesList  = func(recipientID string) string { return "/recipients/" + url.PathEscape(recipientID) + "/preferences" }
	RecipientsPreferencesSet   = func(recipientID string) string { return "/recipients/" + url.PathEscape(recipientID) + "/preferences" }
	RecipientsPreferencesCheck = func(recipientID string) string {
		return "/recipients/" + url.PathEscape(recipientID) + "/preferences/check"
	}

	RecipientsContactsList = func(recipientID string) string {
		return "/recipients/" + url.PathEscape(recipientID) + "/contacts"
	}
	RecipientsContactsCreate = func(recipientID string) string {
		return "/recipients/" + url.PathEscape(recipientID) + "/contacts"
	}
	// SetPrimary is the idempotent "ensure this is the primary contact for this
	// medium" upsert (PUT) — same collection path as Create.
	RecipientsContactsSetPrimary = func(recipientID string) string {
		return "/recipients/" + url.PathEscape(recipientID) + "/contacts"
	}
	RecipientsContactsUpdate = func(recipientID string, contactID int64) string {
		return "/recipients/" + url.PathEscape(recipientID) + "/contacts/" + strconv.FormatInt(contactID, 10)
	}
	RecipientsContactsDelete = func(recipientID string, contactID int64) string {
		return "/recipients/" + url.PathEscape(recipientID) + "/contacts/" + strconv.FormatInt(contactID, 10)
	}
)

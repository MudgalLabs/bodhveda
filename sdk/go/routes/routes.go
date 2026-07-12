// Package routes defines API endpoint routes for the Bodhveda.
package routes

import (
	"net/url"
	"strconv"
)

var (
	NotificationsSend = "/notifications/send"

	RecipientsCreate      = "/recipients"
	RecipientsCreateBatch = "/recipients/batch"
	RecipeientsGet        = func(recipientID string) string { return "/recipients/" + url.PathEscape(recipientID) }
	RecipeientsUpdate     = func(recipientID string) string { return "/recipients/" + url.PathEscape(recipientID) }
	RecipeientsDelete     = func(recipientID string) string { return "/recipients/" + url.PathEscape(recipientID) }

	RecipientsNotificationsList = func(recipientID string) string {
		return "/recipients/" + url.PathEscape(recipientID) + "/notifications"
	}
	RecipientsNotificationUnreadCount = func(recipientID string) string {
		return "/recipients/" + url.PathEscape(recipientID) + "/notifications/unread_count"
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
	RecipientsContactsUpdate = func(recipientID string, contactID int64) string {
		return "/recipients/" + url.PathEscape(recipientID) + "/contacts/" + strconv.FormatInt(contactID, 10)
	}
	RecipientsContactsDelete = func(recipientID string, contactID int64) string {
		return "/recipients/" + url.PathEscape(recipientID) + "/contacts/" + strconv.FormatInt(contactID, 10)
	}
)

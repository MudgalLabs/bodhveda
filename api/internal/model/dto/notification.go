package dto

import "encoding/json"

type Notification struct{}

type SendNotificationPayload struct {
	ProjectID int

	Payload json.RawMessage `json:"payload"`
}

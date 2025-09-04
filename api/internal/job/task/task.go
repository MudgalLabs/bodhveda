// Package task defines constants for different types of background tasks used in the application.
package task

const (
	TaskTypeNotificationDelivery    = "notification:delivery"
	TaskTypePrepareBroadcastBatches = "broadcast:prepare_batches"
	TaskTypeBroadcastDelivery       = "broadcast:delivery"
	TaskTypeDeleteRecipientData     = "recipient:delete_data"
	TaskTypeDeleteProjectData       = "project:delete_data"
)

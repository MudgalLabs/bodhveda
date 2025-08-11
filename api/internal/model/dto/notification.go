package dto

import (
	"encoding/json"
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/tantra/apires"
	"github.com/mudgallabs/tantra/service"
)

type Notification struct {
	ID             int             `json:"id"`
	RecipientExtID string          `json:"recipient_id"`
	Payload        json.RawMessage `json:"payload"`
	BroadcastID    *int            `json:"broadcast_id"`
	Channel        string          `json:"channel"`
	Topic          string          `json:"topic"`
	Event          string          `json:"event"`
	Read           bool            `json:"read"`
	Opened         bool            `json:"opened"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

func FromNotification(notification *entity.Notification) *Notification {
	dto := &Notification{
		ID:             notification.ID,
		RecipientExtID: notification.RecipientExtID,
		Payload:        notification.Payload,
		BroadcastID:    notification.BroadcastID,
		Channel:        notification.Channel,
		Topic:          notification.Topic,
		Event:          notification.Event,
		CreatedAt:      notification.CreatedAt,
		UpdatedAt:      notification.UpdatedAt,
	}

	if notification.ReadAt != nil {
		dto.Read = true
	}

	if notification.OpenedAt != nil {
		dto.Opened = true
	}

	return dto
}

type Target struct {
	Channel string `json:"channel"`
	// Cannot be "any" as that's reserved for preferences and it makes no sense to
	// send notifications to "any" topic. Although "none" is allowed.
	Topic string `json:"topic"`
	Event string `json:"event"`
}

func TargetFromBroadcast(broadcast *entity.Broadcast) Target {
	return Target{
		Channel: broadcast.Channel,
		Topic:   broadcast.Topic,
		Event:   broadcast.Event,
	}
}

func TargetFromNotification(notification *entity.Notification) Target {
	return Target{
		Channel: notification.Channel,
		Topic:   notification.Topic,
		Event:   notification.Event,
	}
}

func TargetFromPreference(pref *entity.Preference) Target {
	return Target{
		Channel: pref.Channel,
		Topic:   pref.Topic,
		Event:   pref.Event,
	}
}

type SendNotificationPayload struct {
	ProjectID int

	// RecipientExtID is the ID of the recipient for the notification.
	// Optional, if nil then it's a broadcast notification, if present then it's a direct notification.
	RecipientExtID *string `json:"recipient_id"`

	// Optional for direct notifications, but required for broadcast notifications.
	Target *Target `json:"target"`

	// Payload is the actual notification payload.
	// TODO: Add a 4KB limit to this field.
	Payload json.RawMessage `json:"payload"`
}

func (p *SendNotificationPayload) Validate() error {
	var errs service.InputValidationErrors

	if p.ProjectID <= 0 {
		errs.Add(apires.NewApiError("Project is required", "Project ID must be a positive integer", "project_id", p.ProjectID))
	}

	if p.RecipientExtID != nil && *p.RecipientExtID == "" {
		errs.Add(apires.NewApiError("Recipient ID cannot be empty if provided", "Recipient ID cannot be empty if this field is provided. Omit the field if you want to send a broadcast notification.", "recipient_id", p.RecipientExtID))
	}

	// If RecipientExtID is nil, then it's a broadcast notification.
	// We need to ensure that valid channel, topic, and event are provided, if this is a broadcast notification
	// OR even if it's a direct notification, but a value was provided for channel/topic/event.
	if p.RecipientExtID == nil || (p.Target.Channel != "" || p.Target.Topic != "" || p.Target.Event != "") {
		if p.Target.Channel == "" {
			errs.Add(apires.NewApiError("Channel is required", "Channel cannot be empty", "channel", p.Target.Channel))
		}

		switch p.Target.Topic {
		case "":
			errs.Add(apires.NewApiError("Topic is required", "Topic cannot be empty", "topic", p.Target.Topic))
		case "any":
			errs.Add(apires.NewApiError("Invalid topic", "Topic cannot be 'any'. It's reserved for creating project preferences.", "topic", p.Target.Topic))
		}

		if p.Target.Event == "" {
			errs.Add(apires.NewApiError("Event is required", "Event cannot be empty", "event", p.Target.Event))
		}
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func (p *SendNotificationPayload) IsDirect() bool {
	return p.RecipientExtID != nil && *p.RecipientExtID != ""
}

func (p *SendNotificationPayload) IsBroadcast() bool {
	return p.RecipientExtID == nil
}

type SendNotificationResult struct {
	// Notification is the notification that was sent.
	// Nil, if this is a broadcast notification.
	// Nil, if the notification was rejected by preferences.
	Notification *Notification `json:"notification"`
	// Broadcast is the broadcast that was sent.
	// Nil, if this is a direct notification.
	Broadcast *Broadcast `json:"broadcast"`
}

type NotificationsOverviewResult struct {
	TotalNotifications int `json:"total_notifications"`
	TotalDirectSent    int `json:"total_direct_sent"`
	TotalBroadcastSent int `json:"total_broadcast_sent"`
}

type NotificationListItem struct {
	Notification
}

func FromNotificationList(notifications []*entity.Notification) []*NotificationListItem {
	if notifications == nil {
		return nil
	}

	dtos := make([]*NotificationListItem, len(notifications))

	for i, n := range notifications {
		notificationDto := FromNotification(n)
		dtos[i] = &NotificationListItem{
			Notification: *notificationDto,
		}
	}

	return dtos
}

type ListRecipientNotificationsRequest struct {
	RecipientExtID string
	Before         string
	Limit          int
}

type NotificationIDsPayload struct {
	NotificationIDs []int `json:"notification_ids"`
}

type PrepareBroadcastBatchesPayload struct {
	Broadcast *entity.Broadcast
}

type BroadcastDeliveryTaskPayload struct {
	ProjectID       int
	BroadcastID     int
	BatchID         int
	RecipientExtIDs []string
	Payload         json.RawMessage
	Channel         string
	Topic           string
	Event           string
}

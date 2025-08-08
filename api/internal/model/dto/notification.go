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
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

func FromNotification(n *entity.Notification) *Notification {
	return &Notification{
		ID:             n.ID,
		RecipientExtID: n.RecipientExtID,
		Payload:        n.Payload,
		BroadcastID:    n.BroadcastID,
		Channel:        n.Channel,
		Topic:          n.Topic,
		Event:          n.Event,
		CreatedAt:      n.CreatedAt,
		UpdatedAt:      n.UpdatedAt,
	}
}

type NotificationTarget struct {
	// RecipientExtID is the ID of the recipient for the notification.
	// Optional, if nil then it's a broadcast notification, if present then it's a direct notification.
	RecipientExtID *string `json:"recipient_id"`
	Channel        string  `json:"channel"`
	// Cannot be "any" as that's reserved for preferences and it makes no sense to
	// send notifications to "any" topic. Although "none" is allowed.
	Topic string `json:"topic"`
	Event string `json:"event"`
}

func TargetFromBroadcast(broadcast *entity.Broadcast) NotificationTarget {
	return NotificationTarget{
		RecipientExtID: nil,
		Channel:        broadcast.Channel,
		Topic:          broadcast.Topic,
		Event:          broadcast.Event,
	}
}

func TargetFromNotification(notification *entity.Notification) NotificationTarget {
	return NotificationTarget{
		RecipientExtID: &notification.RecipientExtID,
		Channel:        notification.Channel,
		Topic:          notification.Topic,
		Event:          notification.Event,
	}
}

func TargetFromPreference(pref *entity.Preference) NotificationTarget {
	return NotificationTarget{
		RecipientExtID: pref.RecipientExtID,
		Channel:        pref.Channel,
		Topic:          pref.Topic,
		Event:          pref.Event,
	}
}

type SendNotificationPayload struct {
	ProjectID int

	To      NotificationTarget `json:"to"`
	Payload json.RawMessage    `json:"payload"`
}

func (p *SendNotificationPayload) Validate() error {
	var errs service.InputValidationErrors

	if p.ProjectID <= 0 {
		errs.Add(apires.NewApiError("Project is required", "Project ID must be a positive integer", "project_id", p.ProjectID))
	}

	if p.To.RecipientExtID != nil && *p.To.RecipientExtID == "" {
		errs.Add(apires.NewApiError("Recipient ID cannot be empty if provided", "Recipient ID cannot be empty if this field is provided. Omit the field if you want to send a broadcast notification.", "recipient_id", p.To.RecipientExtID))
	}

	// If RecipientExtID is nil, then it's a broadcast notification.
	// We need to ensure that valid channel, topic, and event are provided, if this is a broadcast notification
	// OR even if it's a direct notification, but a value was provided for channel/topic/event.
	if p.To.RecipientExtID == nil || (p.To.Channel != "" || p.To.Topic != "" || p.To.Event != "") {
		if p.To.Channel == "" {
			errs.Add(apires.NewApiError("Channel is required", "Channel cannot be empty", "channel", p.To.Channel))
		}

		switch p.To.Topic {
		case "":
			errs.Add(apires.NewApiError("Topic is required", "Topic cannot be empty", "topic", p.To.Topic))
		case "any":
			errs.Add(apires.NewApiError("Invalid topic", "Topic cannot be 'any'", "topic", p.To.Topic))
		}

		if p.To.Event == "" {
			errs.Add(apires.NewApiError("Event is required", "Event cannot be empty", "event", p.To.Event))
		}
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func (p *SendNotificationPayload) IsDirect() bool {
	return p.To.RecipientExtID != nil && *p.To.RecipientExtID != ""
}

func (p *SendNotificationPayload) IsBroadcast() bool {
	return p.To.RecipientExtID == nil
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

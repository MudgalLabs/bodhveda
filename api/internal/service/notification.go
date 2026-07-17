package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/hibiken/asynq"
	"github.com/mudgallabs/bodhveda/internal/email"
	"github.com/mudgallabs/bodhveda/internal/env"
	"github.com/mudgallabs/bodhveda/internal/job/task"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/logger"
	"github.com/mudgallabs/tantra/query"
	tantraRepo "github.com/mudgallabs/tantra/repository"
	"github.com/mudgallabs/tantra/service"
)

type NotificationService struct {
	repo               repository.NotificationRepository
	recipientRepo      repository.RecipientRepository
	preferenceRepo     repository.PreferenceRepository
	broadcastRepo      repository.BroadcastRepository
	broadcastBatchRepo repository.BroadcastBatchRepository
	deliveryRepo       repository.NotificationDeliveryRepository
	contactRepo        repository.RecipientContactRepository
	projectEmailRepo   repository.ProjectEmailSettingsRepository

	billingService   *BillingService
	recipientService *RecipientService

	asynqClient *asynq.Client
}

func NewNotificationService(
	repo repository.NotificationRepository, recipientRepo repository.RecipientRepository,
	preferenceRepo repository.PreferenceRepository, broadcastRepo repository.BroadcastRepository,
	broadcastBatchRepo repository.BroadcastBatchRepository,
	deliveryRepo repository.NotificationDeliveryRepository, contactRepo repository.RecipientContactRepository,
	projectEmailRepo repository.ProjectEmailSettingsRepository,
	billingService *BillingService, recipientService *RecipientService,
	asynqClient *asynq.Client,
) *NotificationService {
	return &NotificationService{
		repo:               repo,
		recipientRepo:      recipientRepo,
		preferenceRepo:     preferenceRepo,
		broadcastRepo:      broadcastRepo,
		broadcastBatchRepo: broadcastBatchRepo,
		deliveryRepo:       deliveryRepo,
		contactRepo:        contactRepo,
		projectEmailRepo:   projectEmailRepo,

		billingService:   billingService,
		recipientService: recipientService,

		asynqClient: asynqClient,
	}
}

func (s *NotificationService) Send(ctx context.Context, userID int, payload dto.SendNotificationPayload) (*dto.SendNotificationResult, string, service.Error, error) {
	err := payload.Validate()
	if err != nil {
		return nil, "", service.ErrInvalidInput, err
	}

	result := &dto.SendNotificationResult{}

	if payload.IsDirect() {
		result.Notification, result.Deliveries, err = s.sendDirectNotification(ctx, userID, payload)
		if err != nil {
			return nil, "", service.ErrInternalServerError, fmt.Errorf("send direct notification: %w", err)
		}
	} else {
		// Check if a project preference exists that matches the target.
		// If not, we should return an error as no recipients would be able to receive this broadcast.
		// Broadcasts are in-app only in v1 (email is direct-only — see the HARD
		// RULE in agent-docs/overview.md), so the catalog precondition is checked
		// against the in_app medium.
		prefExists, err := s.preferenceRepo.DoesProjectPreferenceExist(ctx, payload.ProjectID, *payload.Target, enum.MediumInApp)
		if err != nil {
			return nil, "", service.ErrInternalServerError, fmt.Errorf("check if project preference exists: %w", err)
		}

		if !prefExists {
			return nil, "", service.ErrBadRequest, errors.New("No project preference exists that matches the target. Create a project preference first.")
		}

		result.Broadcast, err = s.sendBroadcastNotification(ctx, userID, payload)
		if err != nil {
			return nil, "", service.ErrInternalServerError, fmt.Errorf("send broadcast notification: %w", err)
		}
	}

	var message string
	if payload.IsDirect() {
		if result.Notification != nil {
			message = fmt.Sprintf("Direct notification sent successfully to recipient %s.", result.Notification.RecipientExtID)
		} else {
			message = "No notification was sent. Recipient's preferences do not allow delivery."
		}
	} else if payload.IsBroadcast() {
		if result.Broadcast != nil {
			message = "Broadcast notification sent successfully. It will be delivered to all elligible recipients."
		}
	}

	return result, message, service.ErrNone, nil
}

func (s *NotificationService) sendDirectNotification(ctx context.Context, userID int, payload dto.SendNotificationPayload) (*dto.Notification, []*dto.NotificationDelivery, error) {
	// This is to ensure that we can send notifications to recipients that are not yet created.
	_, _, err := s.recipientService.CreateIfNotExists(ctx, dto.CreateRecipientPayload{
		ProjectID:  payload.ProjectID,
		ExternalID: *payload.RecipientExtID,
	})

	if err != nil {
		return nil, nil, fmt.Errorf("create recipient: %w", err)
	}

	var channel, topic, event string
	if payload.Target != nil {
		channel = payload.Target.Channel
		topic = payload.Target.Topic
		event = payload.Target.Event
	}

	notification := entity.NewNotification(
		payload.ProjectID,
		*payload.RecipientExtID,
		payload.Payload,
		nil,
		channel,
		topic,
		event,
	)

	notification, err = s.repo.Create(ctx, notification)
	if err != nil {
		return nil, nil, fmt.Errorf("create notification: %w", err)
	}

	// In-app delivery (the inbox write) — byte-for-byte unchanged.
	taskPayload, err := json.Marshal(dto.NotificationDeliveryTaskPayload{UserID: userID, Notification: notification})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal notification delivery task payload: %w", err)
	}

	task := asynq.NewTask(task.TaskTypeNotificationDelivery, taskPayload)

	_, err = s.asynqClient.Enqueue(task, asynq.MaxRetry(5))
	if err != nil {
		return nil, nil, fmt.Errorf("enqueue notification delivery task: %w", err)
	}

	// Email fan-out (additional medium, DIRECT-only). Presence of the `email`
	// block is the sender's intent signal; catalog + per-medium preference +
	// primary contact + configured provider gate the actual send. A failure here
	// NEVER rejects the send (old doc #19) — the outcome is recorded on a
	// notification_delivery row and returned to the caller.
	var deliveries []*dto.NotificationDelivery
	if payload.HasEmail() {
		delivery, ferr := s.fanOutEmail(ctx, notification, payload.Email)
		if ferr != nil {
			// Best-effort: log and continue. The send still succeeded in-app.
			logger.Get().Errorf("email fan-out for notification %d: %v", notification.ID, ferr)
		}
		if delivery != nil {
			deliveries = append(deliveries, dto.FromNotificationDelivery(delivery))
		}
	}

	return dto.FromNotification(notification), deliveries, nil
}

// fanOutEmail resolves whether email may fire for a direct send and records the
// outcome as a notification_delivery row. When everything passes it creates a
// `pending` row and enqueues the email:delivery task; otherwise it records a
// terminal skip outcome (muted / no_contact / failed) so the outcome is visible
// rather than silently dropped. The returned error is for logging only — it must
// never reject the send.
func (s *NotificationService) fanOutEmail(ctx context.Context, notification *entity.Notification, email *dto.EmailContent) (*entity.NotificationDelivery, error) {
	projectID := notification.ProjectID
	recipientExtID := notification.RecipientExtID
	target := dto.TargetFromNotification(notification)

	newRow := func(status enum.DeliveryStatus, reason string) *entity.NotificationDelivery {
		d := entity.NewNotificationDelivery(notification.ID, projectID, recipientExtID, enum.MediumEmail, status)
		if reason != "" {
			d.FailureReason = &reason
		}
		return d
	}

	record := func(d *entity.NotificationDelivery) (*entity.NotificationDelivery, error) {
		created, err := s.deliveryRepo.Create(ctx, d)
		if err != nil {
			return nil, fmt.Errorf("create email delivery row: %w", err)
		}
		return created, nil
	}

	// 1. Catalog + per-medium preference gate. For a non-in_app medium this
	//    defaults to NOT deliver unless the target is cataloged (a project-level
	//    row exists) or the recipient explicitly enabled it.
	shouldDeliver, err := s.preferenceRepo.ShouldDirectNotificationBeDelivered(ctx, projectID, recipientExtID, target, enum.MediumEmail)
	if err != nil {
		return record(newRow(enum.DeliveryFailed, "gating_error"))
	}
	if !shouldDeliver {
		// Distinguish "no catalog entry" from "explicitly disabled" for visibility.
		reason := "preference_disabled"
		cataloged, cerr := s.preferenceRepo.DoesProjectPreferenceExist(ctx, projectID, target, enum.MediumEmail)
		if cerr == nil && !cataloged {
			reason = "not_cataloged"
		}
		return record(newRow(enum.DeliverySkippedMuted, reason))
	}

	// 2. Configured provider. Without email settings the project can't send.
	settings, err := s.projectEmailRepo.Get(ctx, projectID)
	if err != nil {
		if errors.Is(err, tantraRepo.ErrNotFound) {
			return record(newRow(enum.DeliveryFailed, "provider_not_configured"))
		}
		return record(newRow(enum.DeliveryFailed, "provider_lookup_error"))
	}

	// 3. Primary email contact.
	contact, err := s.contactRepo.GetPrimary(ctx, projectID, recipientExtID, enum.MediumEmail)
	if err != nil {
		if errors.Is(err, tantraRepo.ErrNotFound) {
			return record(newRow(enum.DeliverySkippedNoContact, ""))
		}
		return record(newRow(enum.DeliveryFailed, "contact_lookup_error"))
	}

	// 4. Everything passed — record a pending row and enqueue the send.
	provider := string(settings.Provider)
	pending := newRow(enum.DeliveryPending, "")
	pending.ContactID = &contact.ID
	pending.AddressSnapshot = &contact.Address
	pending.Provider = &provider

	created, err := record(pending)
	if err != nil {
		return nil, err
	}

	// Build the one-click unsubscribe URL (Phase 6). Best-effort: if the token
	// can't be built the email still sends, just without the List-Unsubscribe
	// header. The token identifies (project, recipient, target); the endpoint
	// re-derives + verifies it (no DB row).
	unsubscribeURL := s.buildUnsubscribeURL(projectID, recipientExtID, target)

	taskPayload, err := json.Marshal(dto.EmailDeliveryTaskPayload{
		DeliveryID:     created.ID,
		ProjectID:      projectID,
		To:             contact.Address,
		Subject:        email.Subject,
		HTML:           email.HTML,
		Text:           email.ResolvedText(),
		UnsubscribeURL: unsubscribeURL,
	})
	if err != nil {
		s.markDeliveryFailed(ctx, created.ID, "enqueue_marshal_error")
		created.Status = enum.DeliveryFailed
		return created, fmt.Errorf("marshal email delivery task payload: %w", err)
	}

	emailTask := asynq.NewTask(task.TaskTypeEmailDelivery, taskPayload)
	if _, err := s.asynqClient.Enqueue(emailTask, asynq.MaxRetry(5)); err != nil {
		s.markDeliveryFailed(ctx, created.ID, "enqueue_error")
		created.Status = enum.DeliveryFailed
		return created, fmt.Errorf("enqueue email delivery task: %w", err)
	}

	return created, nil
}

// markDeliveryFailed flips a pending delivery row to failed when enqueue fails
// after the row was created (best-effort; logs on error).
func (s *NotificationService) markDeliveryFailed(ctx context.Context, deliveryID int64, reason string) {
	err := s.deliveryRepo.UpdateResult(ctx, deliveryID, repository.NotificationDeliveryResult{
		Status:        enum.DeliveryFailed,
		FailureReason: &reason,
		Attempt:       1,
	})
	if err != nil {
		logger.Get().Errorf("mark email delivery %d failed: %v", deliveryID, err)
	}
}

// buildUnsubscribeURL signs a Phase 6 unsubscribe token for (project, recipient,
// target) and returns the public one-click URL. Returns "" (no header injected) if
// the token can't be built or no API base URL is configured — the email still
// sends, just without List-Unsubscribe.
func (s *NotificationService) buildUnsubscribeURL(projectID int, recipientExtID string, target dto.Target) string {
	if env.APIURL == "" {
		return ""
	}
	token, err := email.BuildUnsubscribeToken(email.UnsubscribeClaims{
		ProjectID:      projectID,
		RecipientExtID: recipientExtID,
		Channel:        target.Channel,
		Topic:          target.Topic,
		Event:          target.Event,
	}, []byte(env.HashKey))
	if err != nil {
		logger.Get().Errorf("build unsubscribe token for project %d recipient %s: %v", projectID, recipientExtID, err)
		return ""
	}
	return email.UnsubscribeURL(env.APIURL, token)
}

func (s *NotificationService) sendBroadcastNotification(ctx context.Context, userID int, payload dto.SendNotificationPayload) (*dto.Broadcast, error) {
	broadcast := entity.NewBroadcast(
		payload.ProjectID,
		payload.Payload,
		payload.Target.Channel,
		payload.Target.Topic,
		payload.Target.Event,
	)

	broadcast, err := s.broadcastRepo.Create(ctx, broadcast)
	if err != nil {
		return nil, fmt.Errorf("create broadcast: %w", err)
	}

	taskPayload, err := json.Marshal(dto.PrepareBroadcastBatchesPayload{UserID: userID, Broadcast: broadcast})
	if err != nil {
		return nil, fmt.Errorf("marshal prepare broadcast batches task payload: %w", err)
	}

	task := asynq.NewTask(task.TaskTypePrepareBroadcastBatches, taskPayload)

	_, err = s.asynqClient.Enqueue(task, asynq.MaxRetry(5))
	if err != nil {
		return nil, fmt.Errorf("enqueue prepare broadcast batches task: %w", err)
	}

	return dto.FromBroadcast(broadcast), nil
}

func (s *NotificationService) Overview(ctx context.Context, projectID int) (*dto.NotificationsOverviewResult, service.Error, error) {
	result, err := s.repo.Overview(ctx, projectID)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("notification repo overview: %w", err)
	}
	return result, service.ErrNone, nil
}

// EmailDeliveryOverview returns the per-status email delivery counts for a
// project, powering the console's email analytics (Phase 5).
func (s *NotificationService) EmailDeliveryOverview(ctx context.Context, projectID int) (*dto.EmailDeliveryOverview, service.Error, error) {
	if projectID <= 0 {
		return nil, service.ErrInvalidInput, fmt.Errorf("projectID required")
	}
	result, err := s.deliveryRepo.EmailDeliveryOverviewForProject(ctx, projectID)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("email delivery overview: %w", err)
	}
	return result, service.ErrNone, nil
}

// ListNotificationDeliveries returns the full delivery records for one
// notification, including each row's provider webhook event history (Phase 9.1).
//
// This is a SEPARATE endpoint from the notifications list on purpose: every
// bounded delivery column rides the list, but provider_response is unbounded (a
// raw provider event body appended per webhook), so it is fetched only when an
// operator opens one delivery. See agent-docs/overview.md, "Phase 9.1 —
// deviations (as built)".
//
// A notification whose send carried no email simply has no delivery rows — that
// is an empty list, not an error.
func (s *NotificationService) ListNotificationDeliveries(ctx context.Context, projectID, notificationID int) (*dto.ListNotificationDeliveriesResult, service.Error, error) {
	if projectID <= 0 {
		return nil, service.ErrInvalidInput, fmt.Errorf("projectID required")
	}
	if notificationID <= 0 {
		return nil, service.ErrInvalidInput, fmt.Errorf("notificationID required")
	}

	// Scoped by projectID: the route only proves the user owns the PROJECT, so the
	// repo must refuse a notification id belonging to someone else's project.
	deliveries, err := s.deliveryRepo.ListForNotification(ctx, projectID, notificationID)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("list deliveries for notification: %w", err)
	}

	result := &dto.ListNotificationDeliveriesResult{
		Deliveries: make([]*dto.NotificationDeliveryDetail, 0, len(deliveries)),
	}

	for _, d := range deliveries {
		result.Deliveries = append(result.Deliveries, dto.FromNotificationDeliveryDetail(d, s.normalizeStoredEvents(d)))
	}

	return result, service.ErrNone, nil
}

// normalizeStoredEvents turns a delivery row's provider_response JSONB array into
// timeline events, reusing the provider adapter's OWN webhook normalizer — the
// same one the inbound webhook path uses (Phase 5). That keeps provider JSON
// shape knowledge inside the adapter, so a future provider adapter stays a
// backend-only change and the console never learns Resend's schema.
//
// Normalization is best-effort presentation: anything unparseable degrades to a
// raw event with an empty Kind rather than failing the request. The Raw body is
// always preserved.
func (s *NotificationService) normalizeStoredEvents(d *entity.NotificationDelivery) []dto.DeliveryEvent {
	if len(d.ProviderResponse) == 0 {
		return nil
	}

	var raws []json.RawMessage
	if err := json.Unmarshal(d.ProviderResponse, &raws); err != nil {
		// Not an array — shouldn't happen (ApplyWebhookStatus always appends to a
		// JSONB array), but surface the payload rather than dropping it.
		logger.Get().Warnw("delivery provider_response is not a JSON array", "delivery_id", d.ID, "error", err)
		return []dto.DeliveryEvent{{Raw: d.ProviderResponse}}
	}

	// The adapter is selected by the row's own provider discriminator. No API key
	// is needed to normalize (the webhook path constructs it the same way).
	var adapter email.Adapter
	if d.Provider != nil {
		a, err := email.NewAdapter(enum.EmailProvider(*d.Provider), "")
		if err != nil {
			logger.Get().Warnw("no adapter for delivery provider", "delivery_id", d.ID, "provider", *d.Provider, "error", err)
		} else {
			adapter = a
		}
	}

	events := make([]dto.DeliveryEvent, 0, len(raws))
	for _, raw := range raws {
		ev := dto.DeliveryEvent{Raw: raw}

		if adapter != nil {
			// Headers are only the Svix idempotency key on the live path; stored
			// events carry none, and the normalizer does not require them.
			if n, err := adapter.NormalizeWebhookEvent(http.Header{}, raw); err == nil {
				ev.Kind = string(n.Kind)
				if !n.At.IsZero() && n.Kind != email.WebhookEventUnknown {
					at := n.At
					ev.At = &at
				}
			}
		}

		events = append(events, ev)
	}

	return events
}

func (s *NotificationService) ListForRecipient(ctx context.Context, projectID int, recipientExtID string, cursor *query.Cursor) ([]*dto.Notification, *query.Cursor, service.Error, error) {
	if recipientExtID == "" {
		return nil, nil, service.ErrInvalidInput, fmt.Errorf("recipient id required")
	}

	err := cursor.Validate(100, 10)
	if err != nil {
		return nil, nil, service.ErrInvalidInput, err
	}

	notifs, returnedCursor, err := s.repo.ListForRecipient(ctx, projectID, recipientExtID, cursor)
	if err != nil {
		return nil, nil, service.ErrInternalServerError, err
	}

	return dto.FromNotifications(notifs), returnedCursor, service.ErrNone, nil
}

func (s *NotificationService) UnreadCountForRecipient(ctx context.Context, projectID int, recipientExtID string) (int, service.Error, error) {
	if recipientExtID == "" {
		return 0, service.ErrInvalidInput, fmt.Errorf("recipient id required")
	}

	count, err := s.repo.UnreadCountForRecipient(ctx, projectID, recipientExtID)
	if err != nil {
		return 0, service.ErrInternalServerError, err
	}

	return count, service.ErrNone, nil
}

func (s *NotificationService) UpdateForRecipient(ctx context.Context, projectID int, recipientExtID string, payload dto.UpdateRecipientNotificationsPayload) (int, service.Error, error) {
	updated, err := s.repo.UpdateForRecipient(ctx, projectID, recipientExtID, payload)
	if err != nil {
		return 0, service.ErrInternalServerError, err
	}

	return updated, service.ErrNone, nil
}

func (s *NotificationService) DeleteForRecipient(ctx context.Context, projectID int, recipientExtID string, notificationIDs []int) (int, service.Error, error) {
	updated, err := s.repo.DeleteForRecipient(ctx, projectID, recipientExtID, notificationIDs)
	if err != nil {
		return 0, service.ErrInternalServerError, err
	}

	return updated, service.ErrNone, nil
}

func (s *NotificationService) ListNotifications(ctx context.Context, payload *dto.ListNotificationsFilters) (*dto.ListNotificationsResult, service.Error, error) {
	payload.Pagination.ApplyDefaults()

	// Validate normalizes too — notably lowercasing the external-id filters,
	// since external ids are stored lowercase (an exact-match filter that
	// doesn't would just never match).
	if err := payload.Validate(); err != nil {
		return nil, service.ErrInvalidInput, err
	}

	notifications, total, err := s.repo.ListNotifications(ctx, payload)
	if err != nil {
		return nil, service.ErrInternalServerError, err
	}

	return &dto.ListNotificationsResult{
		Notifications: dto.FromNotifications(notifications),
		Pagination:    payload.Pagination.GetMeta(total),
	}, service.ErrNone, nil
}

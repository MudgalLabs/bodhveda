package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
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

	// The catalog is a GATEWAY: every medium this send asks for must be cataloged
	// for the target, or nothing is written at all. See gateTarget.
	if svcErr, err := s.gateTarget(ctx, payload.ProjectID, payload.Target, payload.RequestedMediums()); err != nil {
		return nil, "", svcErr, err
	}

	if payload.IsDirect() {
		result.Notification, result.Deliveries, err = s.sendDirectNotification(ctx, userID, payload)
		if err != nil {
			return nil, "", service.ErrInternalServerError, fmt.Errorf("send direct notification: %w", err)
		}
	} else {
		result.Broadcast, err = s.sendBroadcastNotification(ctx, userID, payload)
		if err != nil {
			return nil, "", service.ErrInternalServerError, fmt.Errorf("send broadcast notification: %w", err)
		}
	}

	var message string
	if payload.IsDirect() {
		// The notification row always exists at this point; preference gating,
		// billing, and email fan-out are resolved asynchronously by the worker.
		// Read the outcome back via GET /notifications/{id}.
		message = fmt.Sprintf("Direct notification queued for delivery to recipient %s.", result.Notification.RecipientExtID)
	} else if payload.IsBroadcast() {
		if result.Broadcast != nil {
			message = "Broadcast notification sent successfully. It will be delivered to all elligible recipients."
		}
	}

	return result, message, service.ErrNone, nil
}

// gateTarget enforces strict targets: a send may only name a target the project
// has cataloged, for every medium it is asking for.
//
// ⚠️ THIS REJECTS. It does not record the send as `muted`, and the difference is
// the whole point:
//
//   - `muted` means THE RECIPIENT SAID NO. A legitimate, healthy outcome, mapped
//     to `suppressed` in every rollup so an opt-out never inflates failure counts.
//   - uncataloged means THE CALLER SENT SOMETHING THIS PROJECT DOES NOT DEFINE.
//     Almost always a typo or a missing setup step.
//
// Conflating them recreates exactly the failure class the delivery-feedback work
// exists to close — the caller gets a 200, believes it sent, and nothing ever
// reaches a human. It also makes `suppressed` mean two unrelated things, so the
// delivery tree reports a caller bug as the system working as designed. There is
// real evidence for this: `conversation/reply/customer` sat in production with
// its topic and event transposed, two notifications that nothing could deliver
// and nobody could mute. A 400 would have caught it on the first call.
//
// ⚠️ EXISTENCE, not enabled. A cataloged-but-disabled entry is a real, deliberate
// state ("defined, currently switched off project-wide"), so a send to it is
// accepted and simply reaches nobody — the audience breakdown reports that
// honestly. Only the ABSENCE of a catalog entry is a caller error.
//
// A send with no target at all is not gated: it is not claiming a target, so
// there is nothing to check it against. Such notifications are also unmutable,
// which is its own problem — see agent-docs/strict-targets-design.md.
func (s *NotificationService) gateTarget(ctx context.Context, projectID int, target *dto.Target, mediums []enum.Medium) (service.Error, error) {
	if target == nil {
		return service.ErrNone, nil
	}

	for _, medium := range mediums {
		cataloged, _, err := s.preferenceRepo.LookupCatalogEntry(ctx, projectID, *target, medium)
		if err != nil {
			return service.ErrInternalServerError, fmt.Errorf("lookup catalog entry: %w", err)
		}

		if !cataloged {
			return service.ErrBadRequest, fmt.Errorf(
				"Target %s/%s/%s is not in this project's catalog for the %s medium. "+
					"Create a project preference for it before sending.",
				target.Channel, target.Topic, target.Event, medium,
			)
		}
	}

	return service.ErrNone, nil
}

// sendDirectNotification is the request-path half of a direct send. It does the
// minimum that has to be synchronous — a single notification INSERT so the API
// can return a real notification id — then enqueues the notification:delivery
// job that carries everything else. Recipient upsert, in-app gating/billing, and
// the entire email fan-out all run in the worker (see DeliverDirectNotification),
// so throughput is bounded by one INSERT + one enqueue rather than the old chain
// of recipient upsert + several email-gate lookups on the hot path.
func (s *NotificationService) sendDirectNotification(ctx context.Context, userID int, payload dto.SendNotificationPayload) (*dto.Notification, []*dto.NotificationDelivery, error) {
	var channel, topic, event string
	if payload.Target != nil {
		channel = payload.Target.Channel
		topic = payload.Target.Topic
		event = payload.Target.Event
	}

	// In-app is requested iff the send carried a `payload` block — the same
	// content-block-implies-intent rule that governs email. When it is absent the
	// body is stored as SQL NULL (not JSON `null`), so `payload IS NULL` in the
	// database means exactly "this was an email-only send".
	inApp := payload.HasPayload()

	var body json.RawMessage
	if inApp {
		body = payload.Payload
	}

	// The notification row references the recipient by external-id string, not a
	// FK, so it can be inserted before the recipient row exists — the worker
	// upserts the recipient. external_id is already lowercased by payload.Validate.
	notification := entity.NewNotification(
		payload.ProjectID,
		*payload.RecipientExtID,
		body,
		nil,
		channel,
		topic,
		event,
	)

	// An email-only send's in-app status is terminal at INSERT: there is nothing
	// for the worker to resolve, because the sender never asked for in-app. It is
	// set here rather than left for the worker so the row is never briefly
	// `enqueued` — a state that would make it look like a pending inbox write to
	// anything reading between the INSERT and the job running.
	//
	// The row still exists (suppressed, not missing): the email delivery, the
	// analytics target join, and GET /notifications/{id} all hang off it.
	if !inApp {
		now := time.Now().UTC()
		notification.Status = enum.NotificationStatusNotRequested
		notification.CompletedAt = &now
	}

	notification, err := s.repo.Create(ctx, notification)
	if err != nil {
		return nil, nil, fmt.Errorf("create notification: %w", err)
	}

	// One job does the rest: recipient upsert, in-app inbox write (gating +
	// billing), and email fan-out. The email block rides along so the worker can
	// resolve it — email outcomes are async now and are read back via
	// GET /notifications/{id}, not returned inline.
	taskPayload, err := json.Marshal(dto.NotificationDeliveryTaskPayload{
		UserID:       userID,
		Notification: notification,
		Email:        payload.Email,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal notification delivery task payload: %w", err)
	}

	task := asynq.NewTask(task.TaskTypeNotificationDelivery, taskPayload)

	_, err = s.asynqClient.Enqueue(task, asynq.MaxRetry(5))
	if err != nil {
		return nil, nil, fmt.Errorf("enqueue notification delivery task: %w", err)
	}

	// Deliveries resolve asynchronously in the worker now — nothing to return here.
	return dto.FromNotification(notification), nil, nil
}

// DeliverDirectNotification is the worker-path half of a direct send, run by the
// notification:delivery job. It carries all the work moved off the request path:
//
//  1. upsert the recipient (so it exists for the inbox / recipient list),
//  2. gate the in-app inbox write on preferences + billing and set the status —
//     SKIPPED entirely for an email-only send (one carrying no `payload`), which
//     has no inbox write to gate and whose status is already terminal,
//  3. fan out email (best-effort — a failure here never fails the job).
//
// Returning a non-nil error lets Asynq retry the whole job (MaxRetry). A quota
// rejection is NOT an error — it is a terminal status on the notification row,
// or on the email delivery row when there is no in-app row to carry it.
func (s *NotificationService) DeliverDirectNotification(ctx context.Context, payload dto.NotificationDeliveryTaskPayload) error {
	notification := payload.Notification

	// 1. Ensure the recipient exists. Moved off the request path — a send to a
	//    not-yet-created recipient still auto-creates it, just in the worker.
	_, _, err := s.recipientService.CreateIfNotExists(ctx, dto.CreateRecipientPayload{
		ProjectID:  notification.ProjectID,
		ExternalID: notification.RecipientExtID,
	})
	if err != nil {
		return fmt.Errorf("create recipient: %w", err)
	}

	// Was in-app requested? MUST go through dto.IsJSONContent, not a len() check:
	// a nil payload marshals into this task as `null` and unmarshals back to the
	// 4-byte slice `null`, so a naive check would write the inbox row an
	// email-only send exists to avoid.
	inApp := dto.IsJSONContent(notification.Payload)

	// quotaExceeded records a plan-limit rejection that must still gate email
	// below. For an in-app send it lands on the notification's status scalar; for
	// an email-only send there is no in-app slot to put it in, so it is carried
	// here and written to the delivery row instead.
	quotaExceeded := false

	if inApp {
		// 2. In-app delivery (the inbox write). Gate on the in_app medium (deliver
		//    unless muted), then meter usage. Logic is byte-for-byte the old worker.
		shouldDeliver, err := s.preferenceRepo.ShouldDirectNotificationBeDelivered(ctx, notification.ProjectID, notification.RecipientExtID, dto.TargetFromNotification(notification), enum.MediumInApp)
		if err != nil {
			return err
		}

		if !shouldDeliver {
			notification.Status = enum.NotificationStatusMuted
		} else {
			event := dto.UsageEvent{
				UserID:    payload.UserID,
				ProjectID: notification.ProjectID,
				Metric:    entity.MetricNotifications,
				Amount:    1,
			}

			err := s.billingService.CheckAndConsumeUsage(ctx, event)
			if err != nil {
				if errors.Is(err, enum.ErrQuotaExceeded) {
					notification.Status = enum.NotificationStatusQuotaExceeded
					quotaExceeded = true
				} else {
					return fmt.Errorf("check and consume usage: %w", err)
				}
			} else {
				notification.Status = enum.NotificationStatusDelivered
			}
		}

		now := time.Now().UTC()
		notification.CompletedAt = &now
		notification.UpdatedAt = now

		if err := s.repo.Update(ctx, notification); err != nil {
			return fmt.Errorf("update notification: %w", err)
		}
	} else {
		// 2'. Email-only send: no inbox write, no in-app preference to consult (the
		//     in_app preference is irrelevant to a send that never asked for it),
		//     and no status to resolve — `not_requested` was set at INSERT and must
		//     NOT be overwritten here. The one thing that still applies is billing:
		//     an email-only send is a send, so it meters the same single
		//     `notifications` unit an in-app one does. Metering it is also what
		//     stops email-only from being a complete plan-limit bypass.
		//
		//     Asymmetry worth naming: a MUTED in-app send is not metered, but an
		//     email-only send whose email later turns out muted/no_contact still is
		//     — the meter runs before the email gate. Charging for a send the caller
		//     made is the conservative side of that trade, and moving the meter
		//     inside the email gate would risk double-metering the mixed case.
		event := dto.UsageEvent{
			UserID:    payload.UserID,
			ProjectID: notification.ProjectID,
			Metric:    entity.MetricNotifications,
			Amount:    1,
		}

		if err := s.billingService.CheckAndConsumeUsage(ctx, event); err != nil {
			if errors.Is(err, enum.ErrQuotaExceeded) {
				quotaExceeded = true
			} else {
				return fmt.Errorf("check and consume usage: %w", err)
			}
		}
	}

	// 3. Email fan-out (additional medium, DIRECT-only). Presence of the `email`
	//    block is the sender's intent signal; catalog + per-medium preference +
	//    primary contact + configured provider gate the actual send. Independent
	//    of the in-app outcome above, and a failure here NEVER fails the job (old
	//    doc #19) — the outcome is recorded on a notification_delivery row.
	if payload.Email != nil {
		// An email-only send that blew the quota is rejected here rather than sent,
		// and the rejection is recorded on the delivery row (the notification's
		// status stays `not_requested` — it describes the in-app medium, which was
		// never requested, and must not be overwritten by another medium's outcome).
		//
		// NOTE the deliberate asymmetry with a MIXED send, which still sends its
		// email when over quota: step 3 has always been independent of step 2, so
		// quota does not gate email there. Making the two consistent is a behaviour
		// change to a shipped path and belongs in its own unit; gating the
		// email-only path is not optional, because without it email-only sends
		// would ignore plan limits entirely.
		if quotaExceeded && !inApp {
			d := entity.NewNotificationDelivery(notification.ID, notification.ProjectID, notification.RecipientExtID, enum.MediumEmail, enum.DeliveryQuotaExceeded)
			if _, derr := s.deliveryRepo.Create(ctx, d); derr != nil {
				logger.Get().Errorf("record quota_exceeded email delivery for notification %d: %v", notification.ID, derr)
			}
			return nil
		}

		if _, ferr := s.fanOutEmail(ctx, notification, payload.Email); ferr != nil {
			logger.Get().Errorf("email fan-out for notification %d: %v", notification.ID, ferr)
		}
	}

	return nil
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

	if payload.Email != nil {
		broadcast = broadcast.WithEmail(payload.Email.Subject, payload.Email.HTML, payload.Email.ResolvedText())
	}

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

// analyticsTargetLimit caps the per-target breakdown to the most active targets.
// A breakdown chart shows a handful; an unbounded list on a project with
// thousands of distinct targets is a payload nobody reads.
const analyticsTargetLimit = 20

// ProjectAnalytics assembles the console Home page's analytics (Phase 9.5): a
// time-series + breakdowns for one project over a date range, bucketed by day in
// the viewer's timezone `tz`.
//
// In-app and email are aggregated SEPARATELY, over their own tables, and merged
// here — never one GROUP BY over a join, which would drop every in-app-only
// notification (still the common case). See dto.ProjectAnalytics.
func (s *NotificationService) ProjectAnalytics(ctx context.Context, filters *dto.AnalyticsFilters, tz string) (*dto.ProjectAnalytics, service.Error, error) {
	if filters.ProjectID <= 0 {
		return nil, service.ErrInvalidInput, fmt.Errorf("projectID required")
	}
	if err := filters.Validate(); err != nil {
		return nil, service.ErrInvalidInput, err
	}
	if tz == "" {
		tz = "UTC"
	}

	// In-app side: aggregate the `notification` row's status scalar. Totals and
	// the per-status split are summed from the (day-bounded) series rather than
	// paying a second scan.
	inAppSeries, err := s.repo.InAppAnalyticsSeries(ctx, filters.ProjectID, filters.CreatedFrom, filters.CreatedTo, tz)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("in-app analytics series: %w", err)
	}
	inApp := dto.AnalyticsInApp{Series: inAppSeries}
	for _, d := range inAppSeries {
		inApp.Total += d.Total
		inApp.ByStatus.Enqueued += d.Enqueued
		inApp.ByStatus.Muted += d.Muted
		inApp.ByStatus.Delivered += d.Delivered
		inApp.ByStatus.QuotaExceeded += d.QuotaExceeded
		inApp.ByStatus.Failed += d.Failed
		inApp.ByStatus.NotRequested += d.NotRequested
	}

	// Email side: aggregate notification_delivery WHERE medium='email'. The repo
	// returns both the per-day series and the summed totals from one scan.
	_, email, err := s.deliveryRepo.EmailAnalyticsSeries(ctx, filters.ProjectID, filters.CreatedFrom, filters.CreatedTo, tz)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("email analytics series: %w", err)
	}

	// Per-target breakdown: notification volumes (all targets, top N), then merge
	// email stats onto those present. Email deliveries are a subset of
	// notifications, so a target with email always has ≥ that many notifications
	// and ranks at least as high — email stats for targets outside the top N are
	// dropped with their (smaller) notification counts, which is the intended
	// "top targets by volume" reading.
	targets, err := s.repo.TargetVolumes(ctx, filters.ProjectID, filters.CreatedFrom, filters.CreatedTo, analyticsTargetLimit)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("target volumes: %w", err)
	}
	emailTargets, err := s.deliveryRepo.EmailTargetStats(ctx, filters.ProjectID, filters.CreatedFrom, filters.CreatedTo)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("email target stats: %w", err)
	}

	type targetKey struct{ channel, topic, event string }
	idx := make(map[targetKey]int, len(targets))
	for i, t := range targets {
		idx[targetKey{t.Channel, t.Topic, t.Event}] = i
	}
	for _, et := range emailTargets {
		if i, ok := idx[targetKey{et.Channel, et.Topic, et.Event}]; ok {
			targets[i].EmailAttempted = et.EmailAttempted
			targets[i].EmailDelivered = et.EmailDelivered
			targets[i].EmailBounced = et.EmailBounced
			targets[i].EmailComplained = et.EmailComplained
		}
	}

	return &dto.ProjectAnalytics{
		InApp:   inApp,
		Email:   *email,
		Targets: targets,
	}, service.ErrNone, nil
}

// GetNotification returns a single notification by id, scoped to the project, with
// its email-medium delivery outcome attached (nil when the send carried no email).
//
// This is the read-by-id counterpart to the now-fully-async send: the send API
// returns a notification id after one INSERT, and the caller polls this endpoint
// to learn the resolved in-app status and — via Email — whether the email was
// sent/delivered/bounced. It mirrors Resend's GET /emails/{id} (return an id on
// send, look up last_event later). Scoped by projectID so a key cannot read a
// notification belonging to another project.
func (s *NotificationService) GetNotification(ctx context.Context, projectID, notificationID int) (*dto.Notification, service.Error, error) {
	if projectID <= 0 {
		return nil, service.ErrInvalidInput, fmt.Errorf("projectID required")
	}
	if notificationID <= 0 {
		return nil, service.ErrInvalidInput, fmt.Errorf("notificationID required")
	}

	notification, err := s.repo.Get(ctx, projectID, notificationID)
	if err != nil {
		if errors.Is(err, tantraRepo.ErrNotFound) {
			return nil, service.ErrNotFound, fmt.Errorf("notification not found")
		}
		return nil, service.ErrInternalServerError, fmt.Errorf("get notification: %w", err)
	}

	return dto.FromNotification(notification), service.ErrNone, nil
}

// GetDeliveryTree returns the per-medium delivery breakdown for ONE direct
// notification — the same dto.DeliveryTree shape BroadcastService.GetDeliveryTree
// returns for a broadcast.
//
// A direct send IS this tree with a fan-out of one, which is the whole reason the
// shape is shared: it makes a direct send and a broadcast comparable in the
// console instead of two unrelated screens. There is no Audience node — a direct
// send names its recipient, so there is no audience to resolve.
//
// The in_app branch comes from the notification row (in-app has no
// notification_delivery row — Phase 4 deliberately left its state on the
// notification), and the email branch from that row's delivery record. That
// asymmetry is real; the DTO builders name it rather than hiding it.
func (s *NotificationService) GetDeliveryTree(ctx context.Context, projectID, notificationID int) (*dto.DeliveryTree, service.Error, error) {
	notification, errKind, err := s.GetNotification(ctx, projectID, notificationID)
	if err != nil {
		return nil, errKind, err
	}

	tree := &dto.DeliveryTree{
		Kind:   enum.NotificationKindDirect,
		Target: notification.Target,
		// Audience stays nil: a direct send has a named recipient, so there is
		// nothing to resolve. The console renders the recipient instead.
		Mediums: []dto.DeliveryTreeMedium{
			dto.InAppMediumFromRollup(map[enum.NotificationStatus]int{
				notification.Status: 1,
			}),
		},
	}

	// The email branch exists only when the send actually attempted email. An
	// absent branch and a branch with zero in it are different facts, so a
	// payload-only send renders no email row at all rather than an empty one.
	if e := notification.Email; e != nil {
		if medium, ok := dto.EmailMediumFromDelivery(&e.Status); ok {
			tree.Mediums = append(tree.Mediums, medium)
		}
	}

	return tree, service.ErrNone, nil
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

// BroadcastEmailCap is the per-project safety rail on how many recipients one
// broadcast may email. It is read from project_email_settings; this is the
// fallback when the project has no row (in which case email cannot send anyway).
const defaultBroadcastEmailCap = 100

// ResolveBroadcastEmailAudience decides, at prepare time, whether a broadcast's
// email half may fan out — and records that decision on the broadcast.
//
// It runs in the caller's transaction so the decision commits with the batches,
// which is what makes it safe for a retry to trust the stored value rather than
// recomputing against a project whose settings or recipients have since changed.
//
// ⚠️ The cap BLOCKS rather than truncates. Mailing the first N of an over-cap
// audience would pick an arbitrary subset by query order and look like success.
// Blocking is loud, recoverable, and leaves the in-app half untouched — which is
// the whole point of requiring a payload on every broadcast.
func (s *NotificationService) ResolveBroadcastEmailAudience(
	ctx context.Context, tx pgx.Tx, broadcast *entity.Broadcast, eligible []string,
) ([]string, error) {
	if broadcast.Email == nil {
		return nil, nil
	}

	// ⚠️ `eligible` is the EMAIL audience resolved in its own right — every
	// recipient who enabled this target for email — NOT a subset of the in-app
	// audience. Someone who muted in-app but opted into email must still receive
	// the mail; they get a notification row with in-app status `muted` for the
	// delivery row to hang off, exactly as a direct send does.
	var err error

	cap := defaultBroadcastEmailCap
	settings, err := s.projectEmailRepo.Get(ctx, broadcast.ProjectID)
	if err != nil {
		if !errors.Is(err, tantraRepo.ErrNotFound) {
			return nil, fmt.Errorf("get project email settings: %w", err)
		}
		// No email settings at all: nothing can send. Record it as blocked rather
		// than letting the fan-out discover it per recipient.
		if serr := s.broadcastRepo.SetEmailOutcomeTx(ctx, tx, broadcast.ID, len(eligible), "provider_not_configured"); serr != nil {
			return nil, serr
		}
		return nil, nil
	}
	if settings.MaxBroadcastRecipientsForEmail > 0 {
		cap = settings.MaxBroadcastRecipientsForEmail
	}

	if len(eligible) > cap {
		logger.Get().Warnw("broadcast email blocked by recipient cap",
			"broadcast_id", broadcast.ID, "eligible", len(eligible), "cap", cap)

		if serr := s.broadcastRepo.SetEmailOutcomeTx(
			ctx, tx, broadcast.ID, len(eligible), entity.EmailBlockedRecipientCapExceeded,
		); serr != nil {
			return nil, serr
		}
		return nil, nil
	}

	if err := s.broadcastRepo.SetEmailOutcomeTx(ctx, tx, broadcast.ID, len(eligible), ""); err != nil {
		return nil, err
	}

	return eligible, nil
}

// FanOutBroadcastEmail writes the email delivery rows for one batch and returns
// the send tasks to enqueue.
//
// ⚠️ It does NOT enqueue. The rows are written in the caller's transaction, and
// publishing a task that references an uncommitted delivery row would let the
// email worker pick it up before it exists. The caller enqueues after commit.
//
// ⚠️ Recipients who are not email-eligible get NO delivery row, matching how the
// in-app half treats broadcast exclusions (a frozen count, not N materialised
// rows — see agent-docs/delivery-feedback-design.md §3.2). Rows are written only
// for recipients this broadcast intended to mail: `pending` when there is an
// address, `no_contact` when there is not, because that one is actionable.
func (s *NotificationService) FanOutBroadcastEmail(
	ctx context.Context, tx pgx.Tx, broadcast *entity.Broadcast, notifications []*entity.Notification,
) ([]dto.EmailDeliveryTaskPayload, error) {
	// ⚠️ BlockedReason is the authoritative go/no-go, decided once at prepare time
	// and read here rather than re-derived. Re-checking the cap per batch would let
	// a settings change mid-fan-out mail some batches and not others.
	if broadcast.Email == nil || broadcast.Email.BlockedReason != "" || len(notifications) == 0 {
		return nil, nil
	}

	target := dto.Target{Channel: broadcast.Channel, Topic: broadcast.Topic, Event: broadcast.Event}

	notificationsByExtID := make(map[string]*entity.Notification, len(notifications))
	candidates := make([]string, 0, len(notifications))
	for _, n := range notifications {
		notificationsByExtID[n.RecipientExtID] = n
		candidates = append(candidates, n.RecipientExtID)
	}

	eligibleExtIDs, err := s.preferenceRepo.FilterEligibleRecipientsForBroadcast(
		ctx, broadcast.ProjectID, target, enum.MediumEmail, candidates,
	)
	if err != nil {
		return nil, fmt.Errorf("filter email-eligible recipients: %w", err)
	}
	if len(eligibleExtIDs) == 0 {
		return nil, nil
	}

	settings, err := s.projectEmailRepo.Get(ctx, broadcast.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("get project email settings: %w", err)
	}

	contacts, err := s.contactRepo.GetPrimaryForRecipients(ctx, broadcast.ProjectID, eligibleExtIDs, enum.MediumEmail)
	if err != nil {
		return nil, fmt.Errorf("get primary contacts: %w", err)
	}

	provider := string(settings.Provider)

	deliveries := make([]*entity.NotificationDelivery, 0, len(eligibleExtIDs))
	sendable := make([]string, 0, len(eligibleExtIDs))

	for _, extID := range eligibleExtIDs {
		notification, ok := notificationsByExtID[extID]
		if !ok {
			// Eligible for email but no in-app row in this batch — cannot happen
			// while email is the intersection of the in-app audience, but a row
			// with nothing to hang off must never be invented.
			continue
		}

		d := entity.NewNotificationDelivery(
			notification.ID, broadcast.ProjectID, extID, enum.MediumEmail, enum.DeliveryPending,
		)

		contact, hasContact := contacts[extID]
		if !hasContact {
			reason := ""
			d.Status = enum.DeliverySkippedNoContact
			d.FailureReason = &reason
			deliveries = append(deliveries, d)
			continue
		}

		d.ContactID = &contact.ID
		d.AddressSnapshot = &contact.Address
		d.Provider = &provider

		deliveries = append(deliveries, d)
		sendable = append(sendable, extID)
	}

	if err := s.deliveryRepo.BatchCreateTx(ctx, tx, deliveries); err != nil {
		return nil, fmt.Errorf("batch create email deliveries: %w", err)
	}

	byExtID := make(map[string]*entity.NotificationDelivery, len(deliveries))
	for _, d := range deliveries {
		byExtID[d.RecipientExtID] = d
	}

	tasks := make([]dto.EmailDeliveryTaskPayload, 0, len(sendable))
	for _, extID := range sendable {
		d := byExtID[extID]

		tasks = append(tasks, dto.EmailDeliveryTaskPayload{
			DeliveryID:     d.ID,
			ProjectID:      broadcast.ProjectID,
			To:             *d.AddressSnapshot,
			Subject:        broadcast.Email.Subject,
			HTML:           broadcast.Email.HTML,
			Text:           broadcast.Email.Text,
			UnsubscribeURL: s.buildUnsubscribeURL(broadcast.ProjectID, extID, target),
		})
	}

	return tasks, nil
}

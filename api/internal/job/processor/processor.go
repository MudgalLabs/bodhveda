// Package processor contains Asynq task processors for handling various background jobs.
package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/email"
	"github.com/mudgallabs/bodhveda/internal/job/task"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/bodhveda/internal/service"
	"github.com/mudgallabs/tantra/dbx"
	"github.com/mudgallabs/tantra/logger"
)

// currentAttempt returns the 1-based attempt number for the task being processed
// (Asynq reports the 0-based retry count).
func currentAttempt(ctx context.Context) int {
	if count, ok := asynq.GetRetryCount(ctx); ok {
		return count + 1
	}
	return 1
}

// NotificationDeliveryProcessor runs a direct send's worker-path work: recipient
// upsert, the in-app inbox write (preference gate + billing), and email fan-out.
// The request path only inserts the notification row + enqueues this job, so all
// of that logic lives in the service (NotificationService.DeliverDirectNotification)
// and the processor is a thin adapter over it.
type NotificationDeliveryProcessor struct {
	notificationService *service.NotificationService
}

func NewNotificationDeliveryProcessor(notificationService *service.NotificationService) *NotificationDeliveryProcessor {
	return &NotificationDeliveryProcessor{
		notificationService: notificationService,
	}
}

func (processor *NotificationDeliveryProcessor) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload dto.NotificationDeliveryTaskPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		err = fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
		logger.Get().Error(err)
		return err
	}

	if err := processor.notificationService.DeliverDirectNotification(ctx, payload); err != nil {
		return err
	}

	logger.Get().Infof("NotificationDeliveryProcessor: Successfully completed notification %d", payload.Notification.ID)

	return nil
}

// EmailDeliveryProcessor sends one email for a direct notification and records
// the outcome on its notification_delivery row. It loads the project's email
// settings fresh (respecting key rotation; no secret rides through Redis),
// builds the provider adapter, sends, and updates the delivery row to sent or
// failed. Email is DIRECT-only — this processor is never invoked for broadcasts.
type EmailDeliveryProcessor struct {
	deliveryRepo     repository.NotificationDeliveryRepository
	projectEmailRepo repository.ProjectEmailSettingsRepository
}

func NewEmailDeliveryProcessor(
	deliveryRepo repository.NotificationDeliveryRepository,
	projectEmailRepo repository.ProjectEmailSettingsRepository,
) *EmailDeliveryProcessor {
	return &EmailDeliveryProcessor{
		deliveryRepo:     deliveryRepo,
		projectEmailRepo: projectEmailRepo,
	}
}

func (processor *EmailDeliveryProcessor) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload dto.EmailDeliveryTaskPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		err = fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
		logger.Get().Error(err)
		return err
	}

	attempt := currentAttempt(ctx)

	// fail records a terminal failed outcome on the delivery row. Returning the
	// original error lets Asynq retry (up to MaxRetry); the row reflects the
	// latest attempt.
	fail := func(reason string, cause error) error {
		r := reason
		updateErr := processor.deliveryRepo.UpdateResult(ctx, payload.DeliveryID, repository.NotificationDeliveryResult{
			Status:        enum.DeliveryFailed,
			FailureReason: &r,
			Attempt:       attempt,
		})
		if updateErr != nil {
			logger.Get().Errorf("EmailDeliveryProcessor: update delivery %d failed status: %v", payload.DeliveryID, updateErr)
		}
		return cause
	}

	settings, err := processor.projectEmailRepo.Get(ctx, payload.ProjectID)
	if err != nil {
		return fail("provider_not_configured", fmt.Errorf("get project email settings: %w", err))
	}

	apiKey, err := settings.DecryptSecret()
	if err != nil {
		return fail("secret_decrypt_error", fmt.Errorf("decrypt provider secret: %w", err))
	}

	adapter, err := email.NewAdapter(settings.Provider, apiKey)
	if err != nil {
		return fail("adapter_init_error", fmt.Errorf("build email adapter: %w", err))
	}

	// RFC 8058 unsubscribe headers (Phase 6). Gmail/Yahoo one-click requires both:
	// the List-Unsubscribe URL Bodhveda hosts + the One-Click POST directive.
	var headers map[string]string
	if payload.UnsubscribeURL != "" {
		headers = map[string]string{
			"List-Unsubscribe":      "<" + payload.UnsubscribeURL + ">",
			"List-Unsubscribe-Post": "List-Unsubscribe=One-Click",
		}
	}

	result, err := adapter.Send(ctx, email.Message{
		FromName:    settings.FromName,
		FromAddress: settings.FromAddress,
		To:          payload.To,
		Subject:     payload.Subject,
		HTML:        payload.HTML,
		Text:        payload.Text,
		Headers:     headers,
		// Stable per-delivery key so an Asynq retry can't send a duplicate email.
		IdempotencyKey: fmt.Sprintf("bodhveda-delivery-%d", payload.DeliveryID),
	})
	if err != nil {
		return fail("provider_send_error", fmt.Errorf("send email: %w", err))
	}

	provider := string(result.Provider)
	messageID := result.ProviderMessageID
	err = processor.deliveryRepo.UpdateResult(ctx, payload.DeliveryID, repository.NotificationDeliveryResult{
		Status:            enum.DeliverySent,
		Provider:          &provider,
		ProviderMessageID: &messageID,
		Attempt:           attempt,
	})
	if err != nil {
		return fmt.Errorf("update delivery sent status: %w", err)
	}

	logger.Get().Infof("EmailDeliveryProcessor: sent email for delivery %d (provider message id %s)", payload.DeliveryID, messageID)
	return nil
}

type PrepareBroadcastBatchesProcessor struct {
	db                 *pgxpool.Pool
	asynqClient        *asynq.Client
	preferenceRepo     repository.PreferenceRepository
	broadcastRepo      repository.BroadcastRepository
	broadcastBatchRepo repository.BroadcastBatchRepository
	billingService     *service.BillingService
	// notificationService owns the email half. Broadcast email resolution lives in
	// the service, not here, for the same reason the direct path does: the
	// processor is a thin adapter over it. Nil is legal and means this deployment
	// resolves no email — every email path checks it.
	notificationService *service.NotificationService
}

func NewPrepareBroadcastBatchesProcessor(
	db *pgxpool.Pool, asynqClient *asynq.Client, preferenceRepo repository.PreferenceRepository,
	broadcastRepo repository.BroadcastRepository, broadcastBatchRepo repository.BroadcastBatchRepository,
	billingService *service.BillingService, notificationService *service.NotificationService,
) *PrepareBroadcastBatchesProcessor {
	return &PrepareBroadcastBatchesProcessor{
		db:                  db,
		asynqClient:         asynqClient,
		preferenceRepo:      preferenceRepo,
		broadcastRepo:       broadcastRepo,
		broadcastBatchRepo:  broadcastBatchRepo,
		billingService:      billingService,
		notificationService: notificationService,
	}
}

func (processor *PrepareBroadcastBatchesProcessor) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload dto.PrepareBroadcastBatchesPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		err = fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
		logger.Get().Error(err)
		return err
	}

	broadcast := payload.Broadcast

	// Broadcasts fan out in-app only (email is direct-only).
	target := dto.TargetFromBroadcast(broadcast)

	// ⚠️ Preparation must be safe to run twice, and it was not.
	//
	// Everything below used to run unguarded on every delivery of this task, so an
	// Asynq retry — after a failed enqueue, a crash, or an expired lease —
	// re-listed the audience, consumed the quota A SECOND TIME, and created a
	// whole new set of batches. Those duplicate batches carry fresh ids, so
	// BroadcastDeliveryProcessor's per-batch guard does not catch them: every one
	// fans out, and every recipient gets the broadcast again.
	//
	// The broadcast row is the guard. Locking it serialises concurrent attempts,
	// and the existence of batches is the durable record that preparation already
	// happened — the two are made the same fact by creating the batches and
	// consuming the usage in ONE transaction.
	//
	// A retry therefore does not re-prepare. It re-enqueues the batches still
	// awaiting delivery, which is safe precisely because delivery is idempotent
	// per batch.
	var batchesToEnqueue []*entity.BroadcastBatch

	err := dbx.WithTx(ctx, processor.db, func(tx pgx.Tx) error {
		status, err := processor.broadcastRepo.StatusForUpdateTx(ctx, tx, broadcast.ID)
		if err != nil {
			return fmt.Errorf("lock broadcast: %w", err)
		}

		// Already finished — completed, or refused for quota. Nothing to prepare
		// and nothing to resume.
		if status != enum.BroadcastStatusEnqueued {
			return nil
		}

		prepared, err := processor.broadcastBatchRepo.CountForBroadcastTx(ctx, tx, broadcast.ID)
		if err != nil {
			return fmt.Errorf("count existing batches: %w", err)
		}

		if prepared > 0 {
			// Preparation committed on an earlier attempt; the audience is already
			// frozen and the quota already consumed. All that can be outstanding is
			// the enqueue, so resume exactly that.
			batchesToEnqueue, err = processor.broadcastBatchRepo.ResumableForBroadcastTx(ctx, tx, broadcast.ID)
			if err != nil {
				return fmt.Errorf("list resumable batches: %w", err)
			}
			return nil
		}

		return processor.prepareTx(ctx, tx, broadcast, target, payload.UserID, &batchesToEnqueue)
	})
	if err != nil {
		logger.Get().Error(err)
		return err
	}

	return processor.enqueueBatches(ctx, broadcast, batchesToEnqueue)
}

// prepareTx does the first-time preparation of a broadcast: freeze the audience,
// consume the quota, and write the batches — all inside the caller's transaction,
// so there is no such thing as a partially prepared broadcast to recover from.
//
// It appends the batches it creates to enqueue, which the caller sends to Asynq
// AFTER the transaction commits. Enqueuing from inside would publish a task
// referencing rows that may still roll back.
func (processor *PrepareBroadcastBatchesProcessor) prepareTx(
	ctx context.Context, tx pgx.Tx, broadcast *entity.Broadcast,
	target dto.Target, userID int, enqueue *[]*entity.BroadcastBatch,
) error {
	recipientExtIDs, err := processor.preferenceRepo.ListEligibleRecipientExtIDsForBroadcast(ctx, broadcast.ProjectID, target, enum.MediumInApp)
	if err != nil {
		return fmt.Errorf("list eligible recipient external IDs: %w", err)
	}

	// Freeze the audience breakdown NOW — this is the only moment these numbers
	// are true. Recomputing them later against a live recipient count is wrong as
	// soon as anyone signs up or leaves. Recorded before the quota check below so
	// a quota-rejected broadcast still shows who it WOULD have reached, which is
	// exactly the question asked when a send does nothing.
	//
	// Best-effort: this is reporting, so a failure here must never fail the
	// fan-out. The tree renders a missing audience as "not recorded".
	if audience, err := processor.preferenceRepo.CountBroadcastAudience(ctx, broadcast.ProjectID, target, enum.MediumInApp); err != nil {
		logger.Get().Errorf("PrepareBroadcastBatchesProcessor: count audience for broadcast %d: %v", broadcast.ID, err)
	} else {
		// Eligible comes from the list we actually fan out to, not the aggregate:
		// the two run as separate queries, so a recipient created between them
		// would make the stored count disagree with the notifications written.
		audience.Eligible = len(recipientExtIDs)

		// ⚠️ Tx variant, not the pool one. We hold this row via FOR UPDATE, so a
		// pool connection would block on our own lock and hang the worker.
		if err := processor.broadcastRepo.SetAudienceTx(ctx, tx, broadcast.ID, audience); err != nil {
			logger.Get().Errorf("PrepareBroadcastBatchesProcessor: set audience for broadcast %d: %v", broadcast.ID, err)
		}
	}

	event := dto.UsageEvent{
		UserID:    userID,
		ProjectID: broadcast.ProjectID,
		Metric:    entity.MetricNotifications,
		Amount:    int64(len(recipientExtIDs)),
	}

	// ⚠️ A broadcast can legitimately match NOBODY — nobody has the target
	// enabled, or the catalog only offers it on another medium. That is a normal
	// outcome (the audience breakdown recorded above is exactly how an operator
	// diagnoses it), not an error.
	//
	// It has to be handled before the usage call, twice over:
	//
	//  1. `usage_log` has CHECK (amount > 0), so consuming 0 units raised
	//     SQLSTATE 23514, the task errored, and Asynq retried it until it was
	//     ARCHIVED. Silent work loss with a failing queue — the exact shape of
	//     the 2026-07-27 incident, reproduced here by an empty audience.
	//  2. With no recipients there are no batches, and it is the LAST BATCH that
	//     marks a broadcast `completed`. So even without the crash the broadcast
	//     would sit in `enqueued` forever, with nothing left to move it.
	//
	// Found by dogfooding on 2026-07-29.
	if len(recipientExtIDs) == 0 {
		now := time.Now().UTC()
		broadcast.Status = enum.BroadcastStatusCompleted
		broadcast.CompletedAt = &now
		broadcast.UpdatedAt = now

		if err := processor.broadcastRepo.UpdateTx(ctx, tx, broadcast); err != nil {
			return fmt.Errorf("update broadcast with empty audience: %w", err)
		}

		logger.Get().Infof("PrepareBroadcastBatchesProcessor: broadcast %d matched no eligible recipients; completed with no batches", broadcast.ID)
		return nil
	}

	// ⚠️ In the caller's transaction, so the usage and the batches below commit as
	// one fact. Consuming in a separate transaction meant a retry after the
	// consume but before the batches billed the same broadcast twice, with
	// nothing recording that it had already been charged.
	if err := processor.billingService.CheckAndConsumeUsageTx(ctx, tx, event); err != nil {
		if !errors.Is(err, enum.ErrQuotaExceeded) {
			return fmt.Errorf("check and consume usage: %w", err)
		}

		now := time.Now().UTC()
		broadcast.Status = enum.BroadcastStatusQuotaExceeded
		broadcast.CompletedAt = &now
		broadcast.UpdatedAt = now

		if err := processor.broadcastRepo.UpdateTx(ctx, tx, broadcast); err != nil {
			return fmt.Errorf("update quota-exceeded broadcast: %w", err)
		}

		logger.Get().Infof("PrepareBroadcastBatchesProcessor: broadcast %d exceeded quota with %d recipients", broadcast.ID, len(recipientExtIDs))
		return nil
	}

	// Decide the email half now, in this transaction, so the decision commits with
	// the batches and a retry can trust the stored value rather than recomputing
	// against settings or an audience that have since moved.
	if broadcast.Email != nil && processor.notificationService != nil {
		if _, err := processor.notificationService.ResolveBroadcastEmailAudience(ctx, tx, broadcast, target, recipientExtIDs); err != nil {
			return fmt.Errorf("resolve broadcast email audience: %w", err)
		}
	}

	var batchSize int

	if len(recipientExtIDs) <= 100 {
		batchSize = len(recipientExtIDs)
	} else {
		batchSize = min(max(len(recipientExtIDs)/10, 100), 1000)
	}

	for i := 0; i < len(recipientExtIDs); i += batchSize {
		end := min(i+batchSize, len(recipientExtIDs))

		broadcastBatch, err := processor.broadcastBatchRepo.CreateTx(ctx, tx, entity.NewBroadcastBatch(broadcast.ID, recipientExtIDs[i:end]))
		if err != nil {
			return fmt.Errorf("create broadcast batch: %w", err)
		}

		*enqueue = append(*enqueue, broadcastBatch)
	}

	logger.Get().Infof("PrepareBroadcastBatchesProcessor: prepared broadcast %d into %d batches for %d recipients",
		broadcast.ID, len(*enqueue), len(recipientExtIDs))

	return nil
}

// enqueueBatches publishes one delivery task per batch, AFTER the preparation
// transaction has committed.
//
// ⚠️ A failure here is not work loss any more. The batches are already durable
// and carry their own recipient ids, so returning the error lets Asynq retry the
// whole task, and the retry re-enqueues exactly the batches still outstanding
// rather than preparing the broadcast again.
func (processor *PrepareBroadcastBatchesProcessor) enqueueBatches(
	ctx context.Context, broadcast *entity.Broadcast, batches []*entity.BroadcastBatch,
) error {
	for _, batch := range batches {
		payload, err := json.Marshal(dto.BroadcastDeliveryTaskPayload{
			ProjectID:       broadcast.ProjectID,
			BroadcastID:     broadcast.ID,
			BatchID:         batch.ID,
			RecipientExtIDs: batch.RecipientExtIDs,
			Payload:         broadcast.Payload,
			Channel:         broadcast.Channel,
			Topic:           broadcast.Topic,
			Event:           broadcast.Event,
		})
		if err != nil {
			err = fmt.Errorf("marshal broadcast delivery task payload: %w", err)
			logger.Get().Error(err)
			return err
		}

		// ⚠️ A stable per-batch task id makes the re-enqueue on a retry a no-op
		// instead of a second copy of the same work. Asynq rejects a duplicate id
		// while the task is still in the queue; once it has run and been retained,
		// the batch's own `success` guard is what stops the redelivery.
		_, err = processor.asynqClient.EnqueueContext(ctx,
			asynq.NewTask(task.TaskTypeBroadcastDelivery, payload),
			asynq.MaxRetry(3),
			asynq.TaskID(fmt.Sprintf("broadcast-batch-%d", batch.ID)),
		)
		if err != nil {
			if errors.Is(err, asynq.ErrTaskIDConflict) {
				// Already queued by a previous attempt — exactly the outcome wanted.
				continue
			}
			err = fmt.Errorf("enqueue broadcast delivery task: %w", err)
			logger.Get().Error(err)
			return err
		}
	}

	return nil
}

type BroadcastDeliveryProcessor struct {
	db                  *pgxpool.Pool
	notificationRepo    repository.NotificationRepository
	broadcastRepo       repository.BroadcastRepository
	broadcastBatchRepo  repository.BroadcastBatchRepository
	notificationService *service.NotificationService
	asynqClient         *asynq.Client
}

func NewBroadcastDeliveryProcessor(
	db *pgxpool.Pool, notificationRepo repository.NotificationRepository,
	broadcastRepo repository.BroadcastRepository, broadcastBatchRepo repository.BroadcastBatchRepository,
	notificationService *service.NotificationService, asynqClient *asynq.Client,
) *BroadcastDeliveryProcessor {
	return &BroadcastDeliveryProcessor{
		db:                  db,
		notificationRepo:    notificationRepo,
		broadcastRepo:       broadcastRepo,
		broadcastBatchRepo:  broadcastBatchRepo,
		notificationService: notificationService,
		asynqClient:         asynqClient,
	}
}

func (processor *BroadcastDeliveryProcessor) ProcessTask(ctx context.Context, t *asynq.Task) error {
	start := time.Now()

	var payload dto.BroadcastDeliveryTaskPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		err = fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
		logger.Get().Error(err)
		return err
	}

	attempt := currentAttempt(ctx)

	// ⚠️ This task must be safe to run twice, and it was not.
	//
	// Asynq is at-least-once: it acks only after ProcessTask returns, so a crash
	// or an expired lease between the commit and the ack re-delivers the batch.
	// The fan-out used to commit in its own transaction and then write the batch
	// status in a SECOND one, so a redelivery re-ran the whole insert and wrote
	// every notification again. Today that is duplicate rows in real inboxes;
	// once broadcasts can carry email it would be duplicate mail to real people.
	//
	// The guard is the batch row itself: lock it, and if it already reads
	// `success` the fan-out is known to have committed, so skip the insert and
	// fall through to the completion check. Locking rather than plainly reading
	// is what makes this hold when two workers hold the same batch at once — the
	// second blocks until the first commits, then sees `success`.
	//
	// A partial unique index on (broadcast_id, recipient_external_id) was the
	// other candidate and is deliberately NOT used: BatchCreateTx inserts via
	// COPY, and COPY has no ON CONFLICT, so a duplicate would abort the batch
	// with an error instead of being absorbed — it would convert a benign retry
	// into a failed one. It would also put a second index on `notification`,
	// which is the send hot path. See agent-docs/delivery-feedback-design.md §3.3.
	var alreadyDelivered bool
	var emailTasks []dto.EmailDeliveryTaskPayload

	// Loaded before the transaction, not inside it: this is the durable record of
	// what to send (content, and whether the email half was blocked at prepare
	// time), and reading it on a pool connection while holding no lock on it
	// cannot contend with anything.
	broadcast, err := processor.broadcastRepo.GetByID(ctx, payload.BroadcastID)
	if err != nil {
		err = fmt.Errorf("get broadcast %d: %w", payload.BroadcastID, err)
		logger.Get().Error(err)
		return err
	}

	err = dbx.WithTx(ctx, processor.db, func(tx pgx.Tx) error {
		status, err := processor.broadcastBatchRepo.StatusForUpdateTx(ctx, tx, payload.BatchID)
		if err != nil {
			return fmt.Errorf("lock broadcast batch: %w", err)
		}

		if status == enum.BroadcastBatchStatusSuccess {
			alreadyDelivered = true
			return nil
		}

		notifications := make([]*entity.Notification, 0, len(payload.RecipientExtIDs))

		// ⚠️ Broadcast notifications are DELIVERED at insert, not `enqueued`.
		//
		// entity.NewNotification defaults to `enqueued` because that is right for
		// a DIRECT send, where notification:delivery still has to resolve
		// preferences and billing afterwards. A broadcast has no such second step:
		// prepare_batches already resolved eligibility and consumed usage, so by
		// the time a row is written it IS in the recipient's inbox and nothing
		// will ever touch it again.
		//
		// Leaving it `enqueued` meant every broadcast notification sat
		// permanently in the only non-terminal status. That was invisible while
		// `recipientFeedVisible` treated `enqueued` as visible (inboxes looked
		// fine), but it made two things wrong the moment they existed: the console
		// delivery tree reported every broadcast as 100% pending, and
		// internal/monitor's stuck_sends check counted all of them as stalled
		// sends and alerted forever. Found by dogfooding on 2026-07-29 — 90 of 90
		// broadcast rows were affected.
		now := time.Now().UTC()

		for _, recipientExtID := range payload.RecipientExtIDs {
			n := entity.NewNotification(
				payload.ProjectID, recipientExtID, payload.Payload,
				&payload.BroadcastID, payload.Channel, payload.Topic, payload.Event,
			)
			n.Status = enum.NotificationStatusDelivered
			n.CompletedAt = &now

			notifications = append(notifications, n)
		}

		if err := processor.notificationRepo.BatchCreateTx(ctx, tx, notifications); err != nil {
			return fmt.Errorf("batch create notifications: %w", err)
		}

		// The email half. Delivery rows are written here, in the same transaction
		// as the notifications they hang off; the send tasks are returned and
		// enqueued only after commit, so no worker can pick up a delivery row that
		// does not exist yet.
		if processor.notificationService != nil {
			emailTasks, err = processor.notificationService.FanOutBroadcastEmail(ctx, tx, broadcast, notifications)
			if err != nil {
				return fmt.Errorf("fan out broadcast email: %w", err)
			}
		}

		// In the SAME transaction as the insert: either the notifications and the
		// batch's `success` both land, or neither does. That is what lets the
		// status above be trusted as the record of whether the work happened.
		return processor.broadcastBatchRepo.UpdateTx(ctx, tx, payload.BatchID, entity.NewBroadcastBatchUpdatePayload(
			enum.BroadcastBatchStatusSuccess, attempt, int(time.Since(start).Milliseconds()),
		))
	})
	if err != nil {
		logger.Get().Error(err)

		// Record the failure OUTSIDE the rolled-back transaction. Best-effort: it
		// is reporting, and losing it must not mask the real error being returned.
		if updateErr := processor.broadcastBatchRepo.Update(ctx, payload.BatchID, entity.NewBroadcastBatchUpdatePayload(
			enum.BroadcastBatchStatusFailed, attempt, int(time.Since(start).Milliseconds()),
		)); updateErr != nil {
			logger.Get().Errorw("update broadcast batch to failed",
				"error", updateErr, "batch_id", payload.BatchID)
		}

		// ⚠️ Return the delivery error so Asynq retries. It used to be overwritten
		// by the batch-status update's own (nil) error and the function returned
		// nil, so a batch that inserted NOTHING was acked as a success and never
		// retried — the failure was recorded in the DB and then dropped.
		return err
	}

	// ⚠️ After commit. A failure here leaves delivery rows `pending` with no task —
	// visible in the tree as stuck rather than silently lost — and returning the
	// error lets Asynq retry, where the batch guard makes the insert a no-op and
	// this loop is reached again.
	if err := processor.enqueueEmailTasks(ctx, emailTasks); err != nil {
		return err
	}

	if alreadyDelivered {
		logger.Get().Infow("broadcast batch already delivered, skipping re-insert",
			"batch_id", payload.BatchID, "broadcast_id", payload.BroadcastID, "attempt", attempt)
	}

	// ⚠️ Runs even when alreadyDelivered. A previous attempt may have committed
	// the fan-out and then died before completing the broadcast, and skipping this
	// on the retry would strand the broadcast in `enqueued` forever — trading the
	// duplicate-insert bug for a never-completes one.
	remaining, err := processor.broadcastBatchRepo.PendingCount(ctx, payload.BroadcastID)
	if err != nil {
		// ⚠️ Must return, not fall through. `remaining` is 0 when the count fails,
		// which reads as "every batch is done" and would complete the broadcast on
		// the strength of a failed query. Retrying is safe now that the batch lock
		// above makes a second run a no-op.
		err = fmt.Errorf("count pending batches: %w", err)
		logger.Get().Error(err)
		return err
	}

	// All batches processed, we can mark the broadcast as completed.
	if remaining == 0 {
		broadcast, err := processor.broadcastRepo.GetByID(ctx, payload.BroadcastID)
		if err != nil {
			// ⚠️ Must return: the next line dereferences `broadcast`, so falling
			// through here panicked the worker on any lookup failure.
			err = fmt.Errorf("get broadcast by ID: %w", err)
			logger.Get().Error(err)
			return err
		}

		now := time.Now().UTC()
		broadcast.CompletedAt = &now
		broadcast.Status = enum.BroadcastStatusCompleted
		broadcast.UpdatedAt = now

		err = processor.broadcastRepo.Update(ctx, broadcast)
		if err != nil {
			err = fmt.Errorf("update broadcast: %w", err)
			logger.Get().Error(err)
			return err
		}

		logger.Get().Infof("BroadcastDeliveryProcessor: Successfully completed broadcast %d", payload.BroadcastID)
	}

	return nil
}

type DeleteRecipientDataProcessor struct {
	db               *pgxpool.Pool
	notificationRepo repository.NotificationRepository
	preferenceRepo   repository.PreferenceRepository
	recipientRepo    repository.RecipientRepository
}

func NewDeleteRecipientDataProcessor(
	preferenceRepo repository.PreferenceRepository, notificationRepo repository.NotificationRepository,
	recipientRepo repository.RecipientRepository,
) *DeleteRecipientDataProcessor {
	return &DeleteRecipientDataProcessor{
		notificationRepo: notificationRepo,
		preferenceRepo:   preferenceRepo,
		recipientRepo:    recipientRepo,
	}
}

func (processor *DeleteRecipientDataProcessor) ProcessTask(ctx context.Context, t *asynq.Task) error {
	l := logger.Get()

	var payload dto.DeleteRecipientDataPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		err = fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
		logger.Get().Error(err)
		return err
	}

	// 1. Delete preferences for the recipient.
	count, err := processor.preferenceRepo.DeleteForRecipient(ctx, payload.ProjectID, payload.RecipientExtID)
	if err != nil {
		err = fmt.Errorf("delete preferences for recipient: %w", err)
		l.Error(err)
		return err
	}
	l.Infof("Deleted %d preferences for recipient %s in project %d", count, payload.RecipientExtID, payload.ProjectID)

	// 2. Delete notifications for the recipient.
	count, err = processor.notificationRepo.DeleteForRecipient(ctx, payload.ProjectID, payload.RecipientExtID, nil)
	if err != nil {
		err = fmt.Errorf("delete notifications for recipient: %w", err)
		l.Error(err)
		return err
	}
	l.Infof("Deleted %d notifications for recipient %s in project %d", count, payload.RecipientExtID, payload.ProjectID)

	// 3. Finally, delete the recipient itself.
	err = processor.recipientRepo.Delete(ctx, payload.ProjectID, payload.RecipientExtID)
	if err != nil {
		err = fmt.Errorf("delete recipient: %w", err)
		l.Error(err)
		return err
	}
	l.Infof("Deleted recipient %s in project %d", payload.RecipientExtID, payload.ProjectID)

	l.Infof("DeleteRecipientDataProcessor: Successfully deleted all data for recipient %s in project %d", payload.RecipientExtID, payload.ProjectID)
	return nil
}

type DeleteProjectDataProcessor struct {
	apikeyRepo         repository.APIKeyRepository
	broadcastRepo      repository.BroadcastRepository
	broadcastBatchRepo repository.BroadcastBatchRepository
	notificationRepo   repository.NotificationRepository
	preferenceRepo     repository.PreferenceRepository
	projectRepo        repository.ProjectRepository
	recipientRepo      repository.RecipientRepository
}

func NewDeleteProjectDataProcessor(
	apikeyRepo repository.APIKeyRepository,
	broadcastRepo repository.BroadcastRepository,
	broadcastBatchRepo repository.BroadcastBatchRepository,
	notificationRepo repository.NotificationRepository,
	preferenceRepo repository.PreferenceRepository,
	projectRepo repository.ProjectRepository,
	recipientRepo repository.RecipientRepository,
) *DeleteProjectDataProcessor {
	return &DeleteProjectDataProcessor{
		apikeyRepo:         apikeyRepo,
		broadcastRepo:      broadcastRepo,
		broadcastBatchRepo: broadcastBatchRepo,
		notificationRepo:   notificationRepo,
		preferenceRepo:     preferenceRepo,
		projectRepo:        projectRepo,
		recipientRepo:      recipientRepo,
	}
}

func (processor *DeleteProjectDataProcessor) ProcessTask(ctx context.Context, t *asynq.Task) error {
	l := logger.Get()

	var payload dto.DeleteProjectDataPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		err = fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
		logger.Get().Error(err)
		return err
	}

	// 1. Delete API keys for the project.
	count, err := processor.apikeyRepo.DeleteForProject(ctx, payload.ProjectID)
	if err != nil {
		err = fmt.Errorf("delete API keys for project: %w", err)
		l.Error(err)
		return err
	}
	l.Infof("Deleted %d API keys for project %d", count, payload.ProjectID)

	// 2. Delete notifications for the project.
	count, err = processor.notificationRepo.DeleteForProject(ctx, payload.ProjectID)
	if err != nil {
		err = fmt.Errorf("delete notifications for project: %w", err)
		l.Error(err)
		return err
	}
	l.Infof("Deleted %d notifications for project %d", count, payload.ProjectID)

	// 3. Delete preferences for the project.
	count, err = processor.preferenceRepo.DeleteForProject(ctx, payload.ProjectID)
	if err != nil {
		err = fmt.Errorf("delete preferences for project: %w", err)
		l.Error(err)
		return err
	}
	l.Infof("Deleted %d preferences for project %d", count, payload.ProjectID)

	// 4. Delete recipients for the project.
	count, err = processor.recipientRepo.DeleteForProject(ctx, payload.ProjectID)
	if err != nil {
		err = fmt.Errorf("delete recipients for project: %w", err)
		l.Error(err)
		return err
	}
	l.Infof("Deleted %d recipients for project %d", count, payload.ProjectID)

	// 5. Delete broadcast batches for the project.
	count, err = processor.broadcastBatchRepo.DeleteForProject(ctx, payload.ProjectID)
	if err != nil {
		err = fmt.Errorf("delete broadcast batches for project: %w", err)
		l.Error(err)
		return err
	}
	l.Infof("Deleted %d broadcast batches for project %d", count, payload.ProjectID)

	// 6. Delete broadcasts for the project.
	count, err = processor.broadcastRepo.DeleteForProject(ctx, payload.ProjectID)
	if err != nil {
		err = fmt.Errorf("delete broadcasts for project: %w", err)
		l.Error(err)
		return err
	}
	l.Infof("Deleted %d broadcasts for project %d", count, payload.ProjectID)

	// 7. Finally, delete the project itself.
	err = processor.projectRepo.Delete(ctx, payload.ProjectID)
	if err != nil {
		err = fmt.Errorf("delete project: %w", err)
		l.Error(err)
		return err
	}
	l.Infof("Deleted project %d", payload.ProjectID)

	l.Infof("DeleteProjectDataProcessor: Successfully deleted all data for project %d", payload.ProjectID)
	return nil
}

// enqueueEmailTasks publishes one email:delivery task per pending delivery row.
//
// ⚠️ Each task carries a stable per-delivery Asynq id so a retry of the batch
// cannot enqueue a second send for the same delivery row. The email adapter also
// sends a per-delivery idempotency key to the provider, so a duplicate would have
// to get past both to become a duplicate email.
func (processor *BroadcastDeliveryProcessor) enqueueEmailTasks(ctx context.Context, tasks []dto.EmailDeliveryTaskPayload) error {
	for _, t := range tasks {
		body, err := json.Marshal(t)
		if err != nil {
			err = fmt.Errorf("marshal email delivery task payload: %w", err)
			logger.Get().Error(err)
			return err
		}

		_, err = processor.asynqClient.EnqueueContext(ctx,
			asynq.NewTask(task.TaskTypeEmailDelivery, body),
			asynq.MaxRetry(3),
			asynq.TaskID(fmt.Sprintf("broadcast-email-delivery-%d", t.DeliveryID)),
		)
		if err != nil {
			if errors.Is(err, asynq.ErrTaskIDConflict) {
				continue
			}
			err = fmt.Errorf("enqueue email delivery task: %w", err)
			logger.Get().Error(err)
			return err
		}
	}

	return nil
}

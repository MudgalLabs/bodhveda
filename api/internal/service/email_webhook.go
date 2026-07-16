package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/mudgallabs/bodhveda/internal/email"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/logger"
	tantraRepo "github.com/mudgallabs/tantra/repository"
	"github.com/mudgallabs/tantra/service"
)

// webhookUnknownRetryWindow bounds how long we ask the provider to retry a webhook
// whose delivery row we can't find yet. The worker writes provider_message_id only
// after the provider send returns, so a very fast webhook can arrive first; asking
// for a retry lets that transient miss self-heal. Past this window the id is almost
// certainly not ours (a test event, or an email sent outside Bodhveda), so we ack
// and stop the retries. Sized to comfortably span Svix's early retry cadence
// (~0s, 5s, 5m) without retrying forever.
const webhookUnknownRetryWindow = 15 * time.Minute

// EmailWebhookService ingests inbound provider delivery webhooks (Phase 5) and
// transitions the matching notification_delivery row. It is provider-agnostic:
// the project's configured provider selects the adapter, which owns both
// signature verification and event normalization, so a new provider slots in
// without changing this service or the public endpoint.
type EmailWebhookService struct {
	projectEmailRepo  repository.ProjectEmailSettingsRepository
	deliveryRepo      repository.NotificationDeliveryRepository
	webhookEventRepo  repository.WebhookEventRepository
	preferenceService *PreferenceService
}

func NewEmailWebhookService(
	projectEmailRepo repository.ProjectEmailSettingsRepository,
	deliveryRepo repository.NotificationDeliveryRepository,
	webhookEventRepo repository.WebhookEventRepository,
	preferenceService *PreferenceService,
) *EmailWebhookService {
	return &EmailWebhookService{
		projectEmailRepo:  projectEmailRepo,
		deliveryRepo:      deliveryRepo,
		webhookEventRepo:  webhookEventRepo,
		preferenceService: preferenceService,
	}
}

// Ingest verifies, normalizes, and applies one inbound webhook event for a
// project. Auth IS the signature: a project with no email settings or no webhook
// secret configured, or a request whose signature does not verify, returns
// ErrUnauthorized (→ 401). Events we don't track, or that reference an unknown
// provider message id, are acknowledged (ErrNone) so the provider stops retrying.
func (s *EmailWebhookService) Ingest(ctx context.Context, projectID int, headers http.Header, body []byte) (service.Error, error) {
	if projectID <= 0 {
		return service.ErrInvalidInput, fmt.Errorf("projectID required")
	}

	settings, err := s.projectEmailRepo.Get(ctx, projectID)
	if err != nil {
		if errors.Is(err, tantraRepo.ErrNotFound) {
			// No email settings ⇒ nothing to verify against; reject.
			return service.ErrUnauthorized, fmt.Errorf("project %d has no email settings", projectID)
		}
		return service.ErrInternalServerError, fmt.Errorf("get project email settings: %w", err)
	}

	if !settings.HasWebhookSecret() {
		return service.ErrUnauthorized, fmt.Errorf("project %d has no webhook secret configured", projectID)
	}

	webhookSecret, err := settings.DecryptWebhookSecret()
	if err != nil {
		return service.ErrInternalServerError, fmt.Errorf("decrypt webhook secret: %w", err)
	}

	// The webhook path only verifies + normalizes; no send API key is needed.
	adapter, err := email.NewAdapter(settings.Provider, "")
	if err != nil {
		return service.ErrInternalServerError, fmt.Errorf("build email adapter: %w", err)
	}

	if err := adapter.VerifyWebhookSignature(webhookSecret, headers, body); err != nil {
		// Includes ErrWebhookSignatureInvalid — auth failure.
		return service.ErrUnauthorized, fmt.Errorf("verify webhook signature: %w", err)
	}

	ev, err := adapter.NormalizeWebhookEvent(headers, body)
	if err != nil {
		return service.ErrInvalidInput, fmt.Errorf("normalize webhook event: %w", err)
	}

	// Events we don't map (e.g. delivery_delayed) are acknowledged and ignored.
	if ev.Kind == email.WebhookEventUnknown {
		return service.ErrNone, nil
	}

	if ev.ProviderMessageID == "" {
		logger.Get().Warnf("email webhook: event %q for project %d has no provider message id; ignoring", ev.Kind, projectID)
		return service.ErrNone, nil
	}

	// Idempotency (#8): dedup on the provider's stable per-event id (svix-id), which
	// is constant across retries. Claiming it here (before ApplyWebhookStatus) stops
	// a replay from re-appending to provider_response and re-running suppression. A
	// provider that supplies no event id (empty) is processed without dedup.
	provider := string(settings.Provider)
	eventID := ev.ProviderEventID
	if eventID != "" {
		claimed, err := s.webhookEventRepo.Claim(ctx, projectID, provider, eventID)
		if err != nil {
			return service.ErrInternalServerError, fmt.Errorf("claim webhook event: %w", err)
		}
		if !claimed {
			logger.Get().Infof("email webhook: duplicate event %q (project %d, kind %q); skipping", eventID, projectID, ev.Kind)
			return service.ErrNone, nil
		}
	}

	update := repository.DeliveryWebhookUpdate{
		ProjectID:         projectID,
		ProviderMessageID: ev.ProviderMessageID,
		Status:            webhookStatusFor(ev.Kind),
		Kind:              string(ev.Kind),
		At:                ev.At,
		RawEvent:          ev.Raw,
	}

	if err := s.deliveryRepo.ApplyWebhookStatus(ctx, update); err != nil {
		if errors.Is(err, tantraRepo.ErrNotFound) {
			// No delivery row matched (project, provider_message_id). This can be a
			// race — the worker persists provider_message_id only after the provider
			// send returns, so a very fast webhook can beat it. If the event is recent,
			// ask the provider to retry (non-2xx → 500) so the miss self-heals once the
			// worker catches up. If it's old, the id genuinely isn't ours (a test event
			// or an email sent outside Bodhveda) — ack so retries stop.
			if time.Since(ev.At) < webhookUnknownRetryWindow {
				// Release the idempotency claim so the retry we're asking for is not
				// mistaken for a duplicate and skipped.
				if eventID != "" {
					if rerr := s.webhookEventRepo.Release(ctx, provider, eventID); rerr != nil {
						logger.Get().Errorf("email webhook: release claim for event %q after retryable miss (project %d): %v", eventID, projectID, rerr)
					}
				}
				logger.Get().Warnf("email webhook: delivery row for provider message id %q not found yet (project %d, kind %q); requesting provider retry", ev.ProviderMessageID, projectID, ev.Kind)
				return service.ErrInternalServerError, fmt.Errorf("delivery row for provider message id %q not persisted yet (project %d)", ev.ProviderMessageID, projectID)
			}
			logger.Get().Warnf("email webhook: no delivery row for provider message id %q (project %d, kind %q); acking stale/unknown event", ev.ProviderMessageID, projectID, ev.Kind)
			return service.ErrNone, nil
		}
		return service.ErrInternalServerError, fmt.Errorf("apply webhook status: %w", err)
	}

	// A `complained` (spam) event suppresses future email to that (recipient,
	// target) by flipping the email medium preference off — the same effect as an
	// explicit unsubscribe (Phase 6). Best-effort: a failure here never fails the
	// webhook ack (the complaint is already recorded on the delivery row). This is
	// target-scoped (matching explicit unsubscribe); address-level suppression
	// across all targets is the old doc's `email_suppression` table, deferred to the
	// managed-email tier.
	if ev.Kind == email.WebhookEventComplained {
		s.suppressEmailAfterComplaint(ctx, projectID, ev.ProviderMessageID)
	}

	logger.Get().Infof("email webhook: applied %q to delivery (provider message id %q, project %d)", ev.Kind, ev.ProviderMessageID, projectID)
	return service.ErrNone, nil
}

// suppressEmailAfterComplaint disables the email medium preference for the
// (recipient, target) behind a complained delivery. Best-effort — logs and returns
// on any error; never fails the webhook.
func (s *EmailWebhookService) suppressEmailAfterComplaint(ctx context.Context, projectID int, providerMessageID string) {
	target, err := s.deliveryRepo.GetTargetByProviderMessageID(ctx, projectID, providerMessageID)
	if err != nil {
		logger.Get().Warnf("email webhook: could not resolve target for complained delivery (provider message id %q, project %d): %v", providerMessageID, projectID, err)
		return
	}

	payload := dto.PatchRecipientPreferenceTargetPayload{
		Target: dto.PreferenceTarget{Target: dto.Target{
			Channel: target.Channel,
			Topic:   target.Topic,
			Event:   target.Event,
		}},
		Medium: string(enum.MediumEmail),
	}
	payload.State.Enabled = false

	if _, _, err := s.preferenceService.UpdateRecipientPreferenceTarget(ctx, target.ProjectID, target.RecipientExtID, payload); err != nil {
		logger.Get().Errorf("email webhook: failed to suppress email after complaint (project %d, recipient %s): %v", target.ProjectID, target.RecipientExtID, err)
		return
	}

	logger.Get().Infof("email webhook: suppressed email for recipient %s target %s/%s/%s after complaint (project %d)",
		target.RecipientExtID, target.Channel, target.Topic, target.Event, target.ProjectID)
}

// webhookStatusFor maps a normalized event kind to the delivery status it drives.
// Soft signals (opened/clicked) return nil — they stamp a *_at column but never
// change `status` (email "opened" is unreliable; see Apple MPP caveat).
func webhookStatusFor(kind email.WebhookEventKind) *enum.DeliveryStatus {
	var status enum.DeliveryStatus
	switch kind {
	case email.WebhookEventSent:
		status = enum.DeliverySent
	case email.WebhookEventDelivered:
		status = enum.DeliveryDelivered
	case email.WebhookEventBounced:
		status = enum.DeliveryBounced
	case email.WebhookEventComplained:
		status = enum.DeliveryComplained
	default:
		// opened / clicked: soft signal, no status transition.
		return nil
	}
	return &status
}

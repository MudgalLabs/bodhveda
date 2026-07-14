package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/mudgallabs/bodhveda/internal/email"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/logger"
	tantraRepo "github.com/mudgallabs/tantra/repository"
	"github.com/mudgallabs/tantra/service"
)

// EmailWebhookService ingests inbound provider delivery webhooks (Phase 5) and
// transitions the matching notification_delivery row. It is provider-agnostic:
// the project's configured provider selects the adapter, which owns both
// signature verification and event normalization, so a new provider slots in
// without changing this service or the public endpoint.
type EmailWebhookService struct {
	projectEmailRepo  repository.ProjectEmailSettingsRepository
	deliveryRepo      repository.NotificationDeliveryRepository
	preferenceService *PreferenceService
}

func NewEmailWebhookService(
	projectEmailRepo repository.ProjectEmailSettingsRepository,
	deliveryRepo repository.NotificationDeliveryRepository,
	preferenceService *PreferenceService,
) *EmailWebhookService {
	return &EmailWebhookService{
		projectEmailRepo:  projectEmailRepo,
		deliveryRepo:      deliveryRepo,
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

	update := repository.DeliveryWebhookUpdate{
		ProviderMessageID: ev.ProviderMessageID,
		Status:            webhookStatusFor(ev.Kind),
		Kind:              string(ev.Kind),
		At:                ev.At,
		RawEvent:          ev.Raw,
	}

	if err := s.deliveryRepo.ApplyWebhookStatus(ctx, update); err != nil {
		if errors.Is(err, tantraRepo.ErrNotFound) {
			// The message id isn't one of ours (or the row was deleted). Ack so the
			// provider stops retrying — retrying can't make it match.
			logger.Get().Warnf("email webhook: no delivery row for provider message id %q (project %d, kind %q)", ev.ProviderMessageID, projectID, ev.Kind)
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
	target, err := s.deliveryRepo.GetTargetByProviderMessageID(ctx, providerMessageID)
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

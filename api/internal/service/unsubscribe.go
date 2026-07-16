package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/mudgallabs/bodhveda/internal/email"
	"github.com/mudgallabs/bodhveda/internal/env"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/tantra/service"
)

// UnsubscribeService backs the public one-click unsubscribe endpoint (Phase 6). It
// verifies the self-contained signed token, then flips the recipient's EMAIL
// preference for that target OFF via the same write path the authenticated
// console/API toggle uses (PreferenceService.UpdateRecipientPreferenceTarget) — no
// parallel disable path. After the flip, the direct-send email gate
// (ShouldDirectNotificationBeDelivered(..., email)) returns false, so subsequent
// sends record a `muted` delivery with failure_reason=preference_disabled.
//
// It is idempotent: repeated unsubscribes upsert the same disabled preference.
type UnsubscribeService struct {
	preferenceService *PreferenceService
	hashKey           []byte
}

func NewUnsubscribeService(preferenceService *PreferenceService) *UnsubscribeService {
	return &UnsubscribeService{
		preferenceService: preferenceService,
		hashKey:           []byte(env.HashKey),
	}
}

// PreviewEmailUnsubscribe verifies the token WITHOUT mutating anything and returns
// its target, for rendering the GET confirmation page. Keeping GET side-effect-free
// is deliberate: mail scanners, link prefetchers, and List-Unsubscribe header
// fetchers issue GET requests, so flipping the preference on GET would silently
// unsubscribe recipients who never clicked. The actual flip happens on POST
// (UnsubscribeEmail), which is also the RFC 8058 one-click method. Same error
// mapping as UnsubscribeEmail: tampered → ErrInvalidInput (400), expired →
// ErrUnauthorized (401).
func (s *UnsubscribeService) PreviewEmailUnsubscribe(token string) (dto.Target, service.Error, error) {
	claims, err := email.ParseUnsubscribeToken(token, s.hashKey)
	if err != nil {
		if errors.Is(err, email.ErrUnsubscribeTokenExpired) {
			return dto.Target{}, service.ErrUnauthorized, err
		}
		return dto.Target{}, service.ErrInvalidInput, err
	}
	return dto.Target{Channel: claims.Channel, Topic: claims.Topic, Event: claims.Event}, service.ErrNone, nil
}

// UnsubscribeEmail verifies the token and disables the email medium for its
// (project, recipient, target). It returns the target (for the confirmation page).
// A malformed/tampered token maps to ErrInvalidInput (→ 400); an expired token to
// ErrUnauthorized (→ 401).
func (s *UnsubscribeService) UnsubscribeEmail(ctx context.Context, token string) (dto.Target, service.Error, error) {
	claims, err := email.ParseUnsubscribeToken(token, s.hashKey)
	if err != nil {
		if errors.Is(err, email.ErrUnsubscribeTokenExpired) {
			return dto.Target{}, service.ErrUnauthorized, err
		}
		return dto.Target{}, service.ErrInvalidInput, err
	}

	target := dto.Target{
		Channel: claims.Channel,
		Topic:   claims.Topic,
		Event:   claims.Event,
	}

	payload := dto.PatchRecipientPreferenceTargetPayload{
		Target: dto.PreferenceTarget{Target: target},
		Medium: string(enum.MediumEmail),
	}
	payload.State.Enabled = false

	_, errKind, err := s.preferenceService.UpdateRecipientPreferenceTarget(ctx, claims.ProjectID, claims.RecipientExtID, payload)
	if err != nil {
		return dto.Target{}, errKind, fmt.Errorf("disable email preference: %w", err)
	}

	return target, service.ErrNone, nil
}

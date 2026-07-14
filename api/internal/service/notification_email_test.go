package service

import (
	"context"
	"testing"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	tantraRepo "github.com/mudgallabs/tantra/repository"
)

// --- fakes: each embeds the interface (nil) so only overridden methods are safe
// to call. fanOutEmail only touches the methods overridden below. ---

type fakePrefRepo struct {
	repository.PreferenceRepository
	shouldDeliver bool
	cataloged     bool
}

func (f *fakePrefRepo) ShouldDirectNotificationBeDelivered(ctx context.Context, projectID int, recipientExtID string, target dto.Target, medium enum.Medium) (bool, error) {
	return f.shouldDeliver, nil
}

func (f *fakePrefRepo) DoesProjectPreferenceExist(ctx context.Context, projectID int, target dto.Target, medium enum.Medium) (bool, error) {
	return f.cataloged, nil
}

type fakeEmailSettingsRepo struct {
	repository.ProjectEmailSettingsRepository
	settings *entity.ProjectEmailSettings
}

func (f *fakeEmailSettingsRepo) Get(ctx context.Context, projectID int) (*entity.ProjectEmailSettings, error) {
	if f.settings == nil {
		return nil, tantraRepo.ErrNotFound
	}
	return f.settings, nil
}

type fakeContactRepo struct {
	repository.RecipientContactRepository
	contact *entity.RecipientContact
}

func (f *fakeContactRepo) GetPrimary(ctx context.Context, projectID int, recipientExtID string, medium enum.Medium) (*entity.RecipientContact, error) {
	if f.contact == nil {
		return nil, tantraRepo.ErrNotFound
	}
	return f.contact, nil
}

type fakeDeliveryRepo struct {
	repository.NotificationDeliveryRepository
	created *entity.NotificationDelivery
}

func (f *fakeDeliveryRepo) Create(ctx context.Context, d *entity.NotificationDelivery) (*entity.NotificationDelivery, error) {
	d.ID = 1
	f.created = d
	return d, nil
}

func newNotification() *entity.Notification {
	return &entity.Notification{
		ID: 10, ProjectID: 1, RecipientExtID: "user_1",
		Channel: "digest", Topic: "none", Event: "sent",
	}
}

func settings() *entity.ProjectEmailSettings {
	return &entity.ProjectEmailSettings{ProjectID: 1, Provider: enum.EmailProviderResend, FromName: "R", FromAddress: "hey@r.to"}
}

// fanOutEmail's terminal skip outcomes never reach the asynq enqueue, so a nil
// asynqClient is safe for these cases.
func serviceWith(pref *fakePrefRepo, es *fakeEmailSettingsRepo, c *fakeContactRepo, d *fakeDeliveryRepo) *NotificationService {
	return &NotificationService{
		preferenceRepo:   pref,
		projectEmailRepo: es,
		contactRepo:      c,
		deliveryRepo:     d,
	}
}

func TestFanOutEmail_Uncataloged_Muted(t *testing.T) {
	d := &fakeDeliveryRepo{}
	s := serviceWith(&fakePrefRepo{shouldDeliver: false, cataloged: false}, &fakeEmailSettingsRepo{settings: settings()}, &fakeContactRepo{}, d)

	_, err := s.fanOutEmail(context.Background(), newNotification(), &dto.EmailContent{Subject: "s", Text: "t"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.created == nil || d.created.Status != enum.DeliverySkippedMuted {
		t.Fatalf("status = %v, want muted", statusOf(d))
	}
	if d.created.FailureReason == nil || *d.created.FailureReason != "not_cataloged" {
		t.Errorf("reason = %v, want not_cataloged", d.created.FailureReason)
	}
}

func TestFanOutEmail_Disabled_Muted(t *testing.T) {
	d := &fakeDeliveryRepo{}
	s := serviceWith(&fakePrefRepo{shouldDeliver: false, cataloged: true}, &fakeEmailSettingsRepo{settings: settings()}, &fakeContactRepo{}, d)

	_, _ = s.fanOutEmail(context.Background(), newNotification(), &dto.EmailContent{Subject: "s", Text: "t"})
	if d.created == nil || d.created.Status != enum.DeliverySkippedMuted {
		t.Fatalf("status = %v, want muted", statusOf(d))
	}
	if d.created.FailureReason == nil || *d.created.FailureReason != "preference_disabled" {
		t.Errorf("reason = %v, want preference_disabled", d.created.FailureReason)
	}
}

func TestFanOutEmail_NoProviderSettings_Failed(t *testing.T) {
	d := &fakeDeliveryRepo{}
	s := serviceWith(&fakePrefRepo{shouldDeliver: true}, &fakeEmailSettingsRepo{settings: nil}, &fakeContactRepo{contact: &entity.RecipientContact{ID: 5, Address: "u@e.com"}}, d)

	_, _ = s.fanOutEmail(context.Background(), newNotification(), &dto.EmailContent{Subject: "s", Text: "t"})
	if d.created == nil || d.created.Status != enum.DeliveryFailed {
		t.Fatalf("status = %v, want failed", statusOf(d))
	}
	if d.created.FailureReason == nil || *d.created.FailureReason != "provider_not_configured" {
		t.Errorf("reason = %v, want provider_not_configured", d.created.FailureReason)
	}
}

func TestFanOutEmail_NoContact(t *testing.T) {
	d := &fakeDeliveryRepo{}
	s := serviceWith(&fakePrefRepo{shouldDeliver: true}, &fakeEmailSettingsRepo{settings: settings()}, &fakeContactRepo{contact: nil}, d)

	_, _ = s.fanOutEmail(context.Background(), newNotification(), &dto.EmailContent{Subject: "s", Text: "t"})
	if d.created == nil || d.created.Status != enum.DeliverySkippedNoContact {
		t.Fatalf("status = %v, want no_contact", statusOf(d))
	}
}

func statusOf(d *fakeDeliveryRepo) any {
	if d.created == nil {
		return "<no row created>"
	}
	return d.created.Status
}

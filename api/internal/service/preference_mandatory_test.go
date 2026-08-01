package service

import (
	"context"
	"strings"
	"testing"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/service"
)

// --- Minimal fakes. Only the methods the mandatory guard touches are real. ---

type mandatoryPrefRepo struct {
	repository.PreferenceRepository
	mandatory bool
	created   bool
}

func (f *mandatoryPrefRepo) LookupCatalogEntry(ctx context.Context, projectID int, target dto.Target, medium enum.Medium) (bool, bool, error) {
	return true, f.mandatory, nil
}

func (f *mandatoryPrefRepo) Create(ctx context.Context, pref *entity.Preference) (*entity.Preference, error) {
	f.created = true
	pref.ID = 1
	return pref, nil
}

type alwaysExistsRecipientRepo struct {
	repository.RecipientRepository
}

func (f *alwaysExistsRecipientRepo) Exists(ctx context.Context, projectID int, recipientExtID string) (bool, error) {
	return true, nil
}

func upsertPayload() dto.UpsertRecipientPreferencePayload {
	return dto.UpsertRecipientPreferencePayload{
		ProjectID:      1,
		RecipientExtID: "user-1",
		Channel:        "security",
		Topic:          "none",
		Event:          "password_reset",
		Medium:         string(enum.MediumInApp),
		Enabled:        false,
	}
}

// TestMandatoryEntryRefusesRecipientOptOut pins the guard, not the cascade.
//
// Without it the write would SUCCEED and then be silently ignored, because the
// resolution cascade ranks mandatory catalog rows above the recipient's own. A
// toggle that saves and does nothing is the worst outcome available: the
// recipient walks away believing they opted out of a security alert.
func TestMandatoryEntryRefusesRecipientOptOut(t *testing.T) {
	repo := &mandatoryPrefRepo{mandatory: true}
	s := NewProjectPreferenceService(repo, &alwaysExistsRecipientRepo{})

	_, errKind, err := s.UpsertRecipientPreference(context.Background(), upsertPayload())

	if err == nil {
		t.Fatal("opting out of a mandatory entry must be refused, not silently stored")
	}
	if errKind != service.ErrBadRequest {
		t.Fatalf("want ErrBadRequest so it surfaces as a 400, got %v", errKind)
	}
	if repo.created {
		t.Fatal("a row was written for a refused opt-out; the refusal must happen before the write")
	}
	if !strings.Contains(err.Error(), "security/none/password_reset") {
		t.Fatalf("error should name the target so the caller can act on it, got: %v", err)
	}
}

// TestNonMandatoryEntryStillAcceptsOptOut is the control. It fails if the guard
// is over-broad and starts refusing ordinary opt-outs — which would break every
// notification settings screen.
func TestNonMandatoryEntryStillAcceptsOptOut(t *testing.T) {
	repo := &mandatoryPrefRepo{mandatory: false}
	s := NewProjectPreferenceService(repo, &alwaysExistsRecipientRepo{})

	_, _, err := s.UpsertRecipientPreference(context.Background(), upsertPayload())
	if err != nil {
		t.Fatalf("an ordinary opt-out must still be accepted: %v", err)
	}
	if !repo.created {
		t.Fatal("the recipient preference row was never written")
	}
}

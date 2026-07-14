package service

import (
	"bytes"
	"context"
	"testing"

	"github.com/mudgallabs/bodhveda/internal/env"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	tantraRepo "github.com/mudgallabs/tantra/repository"
	tantraService "github.com/mudgallabs/tantra/service"
)

// fakeProjectEmailSettingsRepo is a minimal in-memory repo keyed by project_id.
type fakeProjectEmailSettingsRepo struct {
	store map[int]*entity.ProjectEmailSettings
}

func newFakeProjectEmailSettingsRepo() *fakeProjectEmailSettingsRepo {
	return &fakeProjectEmailSettingsRepo{store: map[int]*entity.ProjectEmailSettings{}}
}

func (f *fakeProjectEmailSettingsRepo) Get(_ context.Context, projectID int) (*entity.ProjectEmailSettings, error) {
	s, ok := f.store[projectID]
	if !ok {
		return nil, tantraRepo.ErrNotFound
	}
	// Return a copy so callers can't mutate our store by reference.
	cp := *s
	return &cp, nil
}

func (f *fakeProjectEmailSettingsRepo) Upsert(_ context.Context, s *entity.ProjectEmailSettings) (*entity.ProjectEmailSettings, error) {
	cp := *s
	f.store[s.ProjectID] = &cp
	saved := *s
	return &saved, nil
}

var _ repository.ProjectEmailSettingsRepository = (*fakeProjectEmailSettingsRepo)(nil)

// withCipherKey sets a valid 32-byte AES key for the duration of a test.
func withCipherKey(t *testing.T) {
	t.Helper()
	prev := env.CipherKey
	env.CipherKey = "0123456789abcdef0123456789abcdef" // 32 bytes → AES-256
	t.Cleanup(func() { env.CipherKey = prev })
}

func TestProjectEmailSettings_EncryptsAtRestAndMasks(t *testing.T) {
	withCipherKey(t)
	ctx := context.Background()
	repo := newFakeProjectEmailSettingsRepo()
	svc := NewProjectEmailSettingsService(repo)

	const plainKey = "re_supersecretkey_1234"

	result, _, err := svc.Upsert(ctx, &dto.UpsertProjectEmailSettingsPayload{
		ProjectID:   1,
		Secret:      plainKey,
		FromName:    "Acme",
		FromAddress: "Hey@Acme.com",
	})
	if err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	// The response must never carry the plaintext, only a last-4 mask.
	if result.SecretMasked == plainKey {
		t.Fatal("masked secret leaked plaintext")
	}
	if want := "••••••••1234"; result.SecretMasked != want {
		t.Fatalf("mask = %q, want %q", result.SecretMasked, want)
	}
	// From address is normalized to lowercase.
	if result.FromAddress != "hey@acme.com" {
		t.Fatalf("from_address = %q, want lowercased", result.FromAddress)
	}
	if result.Provider != "resend" {
		t.Fatalf("provider defaulted to %q, want resend", result.Provider)
	}

	// At rest the stored bytes are ciphertext (not the plaintext), but decrypt
	// back to the original.
	stored := repo.store[1]
	if bytes.Contains(stored.Secret, []byte(plainKey)) {
		t.Fatal("secret stored in plaintext at rest")
	}
	got, err := stored.DecryptSecret()
	if err != nil {
		t.Fatalf("DecryptSecret failed: %v", err)
	}
	if got != plainKey {
		t.Fatalf("decrypted secret = %q, want %q", got, plainKey)
	}
}

func TestProjectEmailSettings_IdentityOnlyUpdateKeepsSecret(t *testing.T) {
	withCipherKey(t)
	ctx := context.Background()
	repo := newFakeProjectEmailSettingsRepo()
	svc := NewProjectEmailSettingsService(repo)

	const plainKey = "re_original_key_abcd"

	if _, _, err := svc.Upsert(ctx, &dto.UpsertProjectEmailSettingsPayload{
		ProjectID: 1, Secret: plainKey, FromName: "Acme", FromAddress: "hey@acme.com",
	}); err != nil {
		t.Fatalf("initial Upsert failed: %v", err)
	}
	firstNonce := append([]byte(nil), repo.store[1].Nonce...)

	// Update identity only, no secret → the existing key must be preserved.
	result, _, err := svc.Upsert(ctx, &dto.UpsertProjectEmailSettingsPayload{
		ProjectID: 1, FromName: "Acme Inc", FromAddress: "team@acme.com",
	})
	if err != nil {
		t.Fatalf("identity-only Upsert failed: %v", err)
	}
	if result.FromName != "Acme Inc" || result.FromAddress != "team@acme.com" {
		t.Fatalf("identity not updated: %+v", result)
	}

	stored := repo.store[1]
	if !bytes.Equal(stored.Nonce, firstNonce) {
		t.Fatal("nonce changed on identity-only update — secret was re-encrypted unexpectedly")
	}
	got, err := stored.DecryptSecret()
	if err != nil {
		t.Fatalf("DecryptSecret failed: %v", err)
	}
	if got != plainKey {
		t.Fatalf("secret changed on identity-only update: got %q", got)
	}
}

func TestProjectEmailSettings_RotateSecret(t *testing.T) {
	withCipherKey(t)
	ctx := context.Background()
	repo := newFakeProjectEmailSettingsRepo()
	svc := NewProjectEmailSettingsService(repo)

	if _, _, err := svc.Upsert(ctx, &dto.UpsertProjectEmailSettingsPayload{
		ProjectID: 1, Secret: "re_old_key_0000", FromName: "Acme", FromAddress: "hey@acme.com",
	}); err != nil {
		t.Fatalf("initial Upsert failed: %v", err)
	}

	const rotated = "re_new_key_9999"
	if _, _, err := svc.Upsert(ctx, &dto.UpsertProjectEmailSettingsPayload{
		ProjectID: 1, Secret: rotated, FromName: "Acme", FromAddress: "hey@acme.com",
	}); err != nil {
		t.Fatalf("rotate Upsert failed: %v", err)
	}

	got, err := repo.store[1].DecryptSecret()
	if err != nil {
		t.Fatalf("DecryptSecret failed: %v", err)
	}
	if got != rotated {
		t.Fatalf("secret not rotated: got %q, want %q", got, rotated)
	}
}

func TestProjectEmailSettings_FirstConfigRequiresSecret(t *testing.T) {
	withCipherKey(t)
	ctx := context.Background()
	svc := NewProjectEmailSettingsService(newFakeProjectEmailSettingsRepo())

	_, errKind, err := svc.Upsert(ctx, &dto.UpsertProjectEmailSettingsPayload{
		ProjectID: 1, FromName: "Acme", FromAddress: "hey@acme.com",
	})
	if err == nil {
		t.Fatal("expected error when configuring without a secret")
	}
	if errKind != tantraService.ErrInvalidInput {
		t.Fatalf("error kind = %v, want ErrInvalidInput", errKind)
	}
}

func TestProjectEmailSettings_GetNotConfigured(t *testing.T) {
	withCipherKey(t)
	ctx := context.Background()
	svc := NewProjectEmailSettingsService(newFakeProjectEmailSettingsRepo())

	result, _, err := svc.Get(ctx, 1)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result for unconfigured project, got %+v", result)
	}
}

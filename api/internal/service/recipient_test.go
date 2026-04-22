package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	tantraRepo "github.com/mudgallabs/tantra/repository"
	"github.com/mudgallabs/tantra/query"
)

// fakeRecipientRepo is a minimal in-memory RecipientRepository covering only
// the methods exercised by RecipientService.CreateIfNotExists and Get. Other
// methods panic; add implementations when a new test needs them.
type fakeRecipientRepo struct {
	// keyed by project_id + "|" + lowercased external_id
	store map[string]*entity.Recipient
	next  int
}

func newFakeRecipientRepo() *fakeRecipientRepo {
	return &fakeRecipientRepo{store: map[string]*entity.Recipient{}}
}

func (f *fakeRecipientRepo) key(projectID int, externalID string) string {
	return fmt.Sprintf("%d|%s", projectID, externalID)
}

func (f *fakeRecipientRepo) Create(_ context.Context, r *entity.Recipient) (*entity.Recipient, error) {
	k := f.key(r.ProjectID, r.ExternalID)
	if _, ok := f.store[k]; ok {
		return nil, tantraRepo.ErrConflict
	}
	f.next++
	saved := *r
	saved.ID = f.next
	f.store[k] = &saved
	return &saved, nil
}

func (f *fakeRecipientRepo) Get(_ context.Context, projectID int, externalID string) (*entity.Recipient, error) {
	r, ok := f.store[f.key(projectID, externalID)]
	if !ok {
		return nil, tantraRepo.ErrNotFound
	}
	return r, nil
}

// unused in these tests — panic so we catch accidental coverage gaps.
func (f *fakeRecipientRepo) List(context.Context, int, query.Pagination) ([]*entity.RecipientListItem, int, error) {
	panic("not implemented")
}
func (f *fakeRecipientRepo) Exists(context.Context, int, string) (bool, error) {
	panic("not implemented")
}
func (f *fakeRecipientRepo) TotalCount(context.Context, int) (int, error) {
	panic("not implemented")
}
func (f *fakeRecipientRepo) BatchCreate(context.Context, []*entity.Recipient) ([]string, []string, error) {
	panic("not implemented")
}
func (f *fakeRecipientRepo) Update(context.Context, int, string, *dto.UpdateRecipientPayload) (*entity.Recipient, error) {
	panic("not implemented")
}
func (f *fakeRecipientRepo) SoftDelete(context.Context, int, string) error {
	panic("not implemented")
}
func (f *fakeRecipientRepo) Delete(context.Context, int, string) error {
	panic("not implemented")
}
func (f *fakeRecipientRepo) DeleteForProject(context.Context, int) (int, error) {
	panic("not implemented")
}

var _ repository.RecipientRepository = (*fakeRecipientRepo)(nil)

func newTestRecipientService(repo repository.RecipientRepository) *RecipientService {
	// asynqClient is nil: CreateIfNotExists does not enqueue work, so this is safe
	// for these tests. Do not call Delete with this service.
	return NewRecipientService(repo, nil)
}

// TestCreateIfNotExists_MixedCaseIdempotent reproduces the production bug:
// two back-to-back CreateIfNotExists calls with the same mixed-case external
// ID used to 500 on the second call because the insert path lowercased but
// the Get path did not.
func TestCreateIfNotExists_MixedCaseIdempotent(t *testing.T) {
	ctx := context.Background()
	svc := newTestRecipientService(newFakeRecipientRepo())

	payload := dto.CreateRecipientPayload{ProjectID: 1, ExternalID: "YmY8QtIRVNBJi3R1x9cPJ4EsGpbAG1wt"}

	first, _, err := svc.CreateIfNotExists(ctx, payload)
	if err != nil {
		t.Fatalf("first CreateIfNotExists failed: %v", err)
	}
	if first.ExternalID != strings.ToLower(payload.ExternalID) {
		t.Fatalf("expected stored ID lowercased, got %q", first.ExternalID)
	}

	// Re-construct the payload with the original mixed case — callers do this.
	payload = dto.CreateRecipientPayload{ProjectID: 1, ExternalID: "YmY8QtIRVNBJi3R1x9cPJ4EsGpbAG1wt"}
	second, _, err := svc.CreateIfNotExists(ctx, payload)
	if err != nil {
		t.Fatalf("second CreateIfNotExists failed: %v", err)
	}
	if second.ExternalID != first.ExternalID {
		t.Fatalf("second call returned different ID: %q vs %q", second.ExternalID, first.ExternalID)
	}
}

// TestCreateIfNotExists_CaseVariantsDeduplicate ensures two different
// case spellings of the same external ID don't create duplicates.
func TestCreateIfNotExists_CaseVariantsDeduplicate(t *testing.T) {
	ctx := context.Background()
	svc := newTestRecipientService(newFakeRecipientRepo())

	lower, _, err := svc.CreateIfNotExists(ctx, dto.CreateRecipientPayload{ProjectID: 1, ExternalID: "abc"})
	if err != nil {
		t.Fatalf("CreateIfNotExists(abc) failed: %v", err)
	}

	upper, _, err := svc.CreateIfNotExists(ctx, dto.CreateRecipientPayload{ProjectID: 1, ExternalID: "ABC"})
	if err != nil {
		t.Fatalf("CreateIfNotExists(ABC) failed: %v", err)
	}

	if lower.ExternalID != upper.ExternalID {
		t.Fatalf("case variants produced different IDs: %q vs %q", lower.ExternalID, upper.ExternalID)
	}
}

// TestGet_FindsRecipientRegardlessOfInputCase verifies the Get path does
// not silently miss when the caller supplies an uppercase variant of an
// external ID that was stored lowercase.
func TestGet_FindsRecipientRegardlessOfInputCase(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRecipientRepo()
	svc := newTestRecipientService(repo)

	if _, _, err := svc.Create(ctx, dto.CreateRecipientPayload{ProjectID: 1, ExternalID: "abc"}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Simulate a handler that has already normalized the URL param.
	got, _, err := svc.Get(ctx, 1, strings.ToLower("ABC"))
	if err != nil {
		t.Fatalf("Get(ABC) failed: %v", err)
	}
	if got.ExternalID != "abc" {
		t.Fatalf("Get returned wrong recipient: %q", got.ExternalID)
	}
}

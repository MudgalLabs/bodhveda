package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	tantraRepo "github.com/mudgallabs/tantra/repository"
	"github.com/mudgallabs/tantra/service"
)

type PreferenceService struct {
	repo          repository.PreferenceRepository
	recipientRepo repository.RecipientRepository
}

func NewProjectPreferenceService(repo repository.PreferenceRepository, recipientRepo repository.RecipientRepository) *PreferenceService {
	return &PreferenceService{
		repo:          repo,
		recipientRepo: recipientRepo,
	}
}

func (s *PreferenceService) CreateProjectPreference(ctx context.Context, payload dto.CreateProjectPreferencePayload) (*dto.ProjectPreference, service.Error, error) {
	if err := payload.Validate(); err != nil {
		return nil, service.ErrInvalidInput, err
	}

	pref := entity.NewPreference(
		&payload.ProjectID,
		nil,
		payload.Channel,
		payload.Topic,
		payload.Event,
		payload.Medium,
		&payload.Name,
		payload.DescriptionPtr(),
		payload.Enabled,
	)

	newPref, err := s.repo.Create(ctx, pref)
	if err != nil {
		if err == tantraRepo.ErrConflict {
			return nil, service.ErrConflict, fmt.Errorf("Preference already exists")
		}
		return nil, service.ErrInternalServerError, fmt.Errorf("repo create preference: %w", err)
	}

	return dto.FromPreferenceForProject(newPref), service.ErrNone, nil
}

// ListProjectPreferencesForAPI lists the project's catalog for the Developer
// API. Unlike the console's ListProjectPreferences it does NOT compute
// per-entry subscriber counts — that is a console dashboard concern and costs a
// query per row. The Dev API just returns the catalog rows themselves.
func (s *PreferenceService) ListProjectPreferencesForAPI(ctx context.Context, projectID int) ([]*dto.ProjectPreference, service.Error, error) {
	prefs, err := s.repo.ListPreferences(ctx, projectID, enum.PreferenceKindProject)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("repo list preferences: %w", err)
	}

	dtos := []*dto.ProjectPreference{}
	for _, pref := range prefs {
		dtos = append(dtos, dto.FromPreferenceForProject(pref))
	}

	return dtos, service.ErrNone, nil
}

// UpsertProjectPreferences declaratively merges a whole desired catalog in one
// call — the primitive a "setup project preferences" script calls once with its
// entire catalog. Each item is upserted by its natural key; with prune, catalog
// rows absent from the set are removed too (merge is the default — see the repo
// method for why prune is opt-in).
//
// An empty set is rejected: with merge it is a pointless no-op, and with prune
// it would wipe the whole catalog — almost certainly a mistake. Deliberate
// removals go through DELETE /preferences/{id}.
func (s *PreferenceService) UpsertProjectPreferences(ctx context.Context, projectID int, items []dto.CreateProjectPreferencePayload, prune bool) ([]*dto.ProjectPreference, service.Error, error) {
	if len(items) == 0 {
		return nil, service.ErrInvalidInput, errors.New("At least one preference is required")
	}

	prefs := make([]*entity.Preference, 0, len(items))
	for i := range items {
		items[i].ProjectID = projectID
		if err := items[i].Validate(); err != nil {
			// Return the structured validation error as-is so it renders as a 422
			// with per-field detail (httpx type-asserts InputValidationErrors).
			return nil, service.ErrInvalidInput, err
		}

		prefs = append(prefs, entity.NewPreference(
			&items[i].ProjectID,
			nil,
			items[i].Channel,
			items[i].Topic,
			items[i].Event,
			items[i].Medium,
			&items[i].Name,
			items[i].DescriptionPtr(),
			items[i].Enabled,
		))
	}

	result, err := s.repo.UpsertProjectPreferences(ctx, projectID, prefs, prune)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("repo upsert project preferences: %w", err)
	}

	dtos := []*dto.ProjectPreference{}
	for _, p := range result {
		dtos = append(dtos, dto.FromPreferenceForProject(p))
	}

	return dtos, service.ErrNone, nil
}

// GetProjectPreference fetches one catalog entry by id, scoped to the project.
// The repo confines the lookup to project-level rows, so an unknown id — or a
// recipient-level row's id — is a 404.
func (s *PreferenceService) GetProjectPreference(ctx context.Context, projectID int, preferenceID int) (*dto.ProjectPreference, service.Error, error) {
	pref, err := s.repo.GetProjectPreferenceByID(ctx, projectID, preferenceID)
	if err != nil {
		if err == tantraRepo.ErrNotFound {
			return nil, service.ErrNotFound, fmt.Errorf("Preference not found")
		}
		return nil, service.ErrInternalServerError, fmt.Errorf("repo get project preference: %w", err)
	}

	return dto.FromPreferenceForProject(pref), service.ErrNone, nil
}

// UpdateProjectPreference updates a catalog entry's name, description and
// project-level default. The natural key is immutable, so only those fields
// change.
func (s *PreferenceService) UpdateProjectPreference(ctx context.Context, projectID int, preferenceID int, payload dto.UpdateProjectPreferencePayload) (*dto.ProjectPreference, service.Error, error) {
	if err := payload.Validate(); err != nil {
		return nil, service.ErrInvalidInput, err
	}

	pref, err := s.repo.UpdateProjectPreference(ctx, projectID, preferenceID, payload.Name, payload.DescriptionPtr(), payload.Enabled)
	if err != nil {
		if err == tantraRepo.ErrNotFound {
			return nil, service.ErrNotFound, fmt.Errorf("Preference not found")
		}
		return nil, service.ErrInternalServerError, fmt.Errorf("repo update project preference: %w", err)
	}

	return dto.FromPreferenceForProject(pref), service.ErrNone, nil
}

// DeleteProjectPreference un-catalogs a (target, medium): it removes the
// project-level row. Scoped to project-level rows in the repo, so a full-scope
// key cannot delete a recipient's own preference by id through this surface.
func (s *PreferenceService) DeleteProjectPreference(ctx context.Context, projectID int, preferenceID int) (service.Error, error) {
	err := s.repo.DeleteProjectPreference(ctx, projectID, preferenceID)
	if err != nil {
		if err == tantraRepo.ErrNotFound {
			return service.ErrNotFound, fmt.Errorf("Preference not found")
		}
		return service.ErrInternalServerError, fmt.Errorf("repo delete project preference: %w", err)
	}

	return service.ErrNone, nil
}

func (s *PreferenceService) ListProjectPreferences(ctx context.Context, projectID int) ([]*dto.ProjectPreferenceListItem, service.Error, error) {
	prefs, err := s.repo.ListPreferences(ctx, projectID, enum.PreferenceKindProject)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("repo list preferences: %w", err)
	}

	dtos := []*dto.ProjectPreferenceListItem{}
	for _, pref := range prefs {
		target := dto.TargetFromPreference(pref)

		// Subscriber count is per (target, medium) — count recipients opted in to
		// this catalog entry's own medium.
		recipients, err := s.repo.ListEligibleRecipientExtIDsForBroadcast(ctx, projectID, target, enum.Medium(pref.Medium))
		if err != nil {
			return nil, service.ErrInternalServerError, fmt.Errorf("repo list eligible recipients: %w", err)
		}

		dtos = append(dtos, &dto.ProjectPreferenceListItem{
			ProjectPreference: *dto.FromPreferenceForProject(pref),
			Subscribers:       len(recipients),
		})
	}

	return dtos, service.ErrNone, nil
}

func (s *PreferenceService) ListRecipientPreferences(ctx context.Context, projectID int) ([]*dto.RecipientPreference, service.Error, error) {
	prefs, err := s.repo.ListPreferences(ctx, projectID, enum.PreferenceKindRecipient)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("repo list preferences: %w", err)
	}

	dtos := []*dto.RecipientPreference{}
	for _, e := range prefs {
		dtos = append(dtos, dto.FromPreferenceForRecipient(e))
	}

	return dtos, service.ErrNone, nil
}

func (s *PreferenceService) UpsertRecipientPreference(ctx context.Context, payload dto.UpsertRecipientPreferencePayload) (*dto.RecipientPreference, service.Error, error) {
	if err := payload.Validate(); err != nil {
		return nil, service.ErrInvalidInput, err
	}

	exists, err := s.recipientRepo.Exists(ctx, payload.ProjectID, payload.RecipientExtID)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("repo check recipient exists: %w", err)
	}

	if !exists {
		return nil, service.ErrNotFound, errors.New("Recipient not found")
	}

	pref := entity.NewPreference(
		&payload.ProjectID,
		&payload.RecipientExtID,
		payload.Channel,
		payload.Topic,
		payload.Event,
		payload.Medium,
		nil,
		nil,
		payload.Enabled,
	)

	pref.UpdatedAt = time.Now().UTC()

	newPref, err := s.repo.Create(ctx, pref)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("repo create preference: %w", err)
	}

	return dto.FromPreferenceForRecipient(newPref), service.ErrNone, nil
}

func (s *PreferenceService) UpdateRecipientPreferenceTarget(ctx context.Context, projectID int, recipientExtID string, req dto.PatchRecipientPreferenceTargetPayload) (*dto.PreferenceTargetStateDTO, service.Error, error) {
	if err := req.Validate(); err != nil {
		return nil, service.ErrInvalidInput, err
	}

	// Upsert recipient-level preference for this (target, medium).
	pref := entity.NewPreference(
		&projectID,
		&recipientExtID,
		req.Target.Channel,
		req.Target.Topic,
		req.Target.Event,
		req.Medium,
		nil,
		nil,
		req.State.Enabled,
	)
	pref.UpdatedAt = time.Now().UTC()

	newPref, err := s.repo.Create(ctx, pref)
	if err != nil {
		if err == tantraRepo.ErrConflict {
			// If already exists, treat as update (should not error)
			// But repo.Create does upsert for recipient-level, so this is unexpected
			return nil, service.ErrConflict, err
		}
		return nil, service.ErrInternalServerError, err
	}

	// Always inherited=false for explicit recipient-level preference
	return dto.PreferenceTargetStateDTOFromPreference(newPref, false), service.ErrNone, nil
}

// GetRecipientProjectPreferences is the Developer API's preference read: every
// (target, Active medium) known for this recipient, resolved by the SAME cascade
// the send path gates on.
//
// It used to be a Go exact-match merge over the project catalog, which disagreed
// with delivery three ways — no topic='any' fallbacks, no medium-dependent
// default, and a recipient's row on an uncataloged pair was invisible while
// still delivering. Customers render their own settings screens off this, so a
// recipient could be shown a toggle that contradicted what they actually
// received. It now shares PreferenceRepo.ResolveRecipientPreferences with the
// console read.
//
// No recipient-exists check: every route reaching this is behind
// CreateRecipientIfNotExists (cmd/api/routes.go), so the recipient is guaranteed
// to exist and a 404 branch would be unreachable — and a wasted query per call.
func (s *PreferenceService) GetRecipientProjectPreferences(ctx context.Context, projectID int, recipientExtID string) (*dto.PreferenceTargetStatesResultDTO, service.Error, error) {
	resolved, err := s.repo.ResolveRecipientPreferences(ctx, projectID, recipientExtID, enum.ActiveMediums())
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("repo resolve recipient preferences: %w", err)
	}

	result := []*dto.PreferenceTargetResolvedStateDTO{}
	for _, p := range resolved {
		result = append(result, dto.FromResolvedPreferenceForAPI(p))
	}

	return &dto.PreferenceTargetStatesResultDTO{
		Preferences: result,
	}, service.ErrNone, nil
}

// ResolveRecipientPreferences answers every known (target, medium) for one
// recipient with the resolution the SEND PATH would perform, resolved in the
// database by the same cascade.
//
// It is the console's read. GetRecipientProjectPreferences is the Developer
// API's, and both now resolve through the same repo cascade — they agree on
// every value. This one exists separately because it adds `source` (the cascade
// rung, which the console renders as prose) and 404s for an unknown recipient.
// Neither belongs on the public surface: `source` would freeze the resolver's
// vocabulary into a permanent contract, and the Dev API's routes auto-create the
// recipient, so a 404 there is unreachable. See the Phase 9.3.1 deviations in
// agent-docs/overview.md.
//
// Only Active mediums are resolved — a toggle for a transport that cannot fire
// would be a lie of a different kind.
func (s *PreferenceService) ResolveRecipientPreferences(ctx context.Context, projectID int, recipientExtID string) (*dto.ResolvedPreferencesResultDTO, service.Error, error) {
	exists, err := s.recipientRepo.Exists(ctx, projectID, recipientExtID)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("repo check recipient exists: %w", err)
	}

	if !exists {
		return nil, service.ErrNotFound, errors.New("Recipient not found")
	}

	resolved, err := s.repo.ResolveRecipientPreferences(ctx, projectID, recipientExtID, enum.ActiveMediums())
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("repo resolve recipient preferences: %w", err)
	}

	dtos := []*dto.ResolvedPreferenceDTO{}
	for _, p := range resolved {
		dtos = append(dtos, dto.FromResolvedPreference(p))
	}

	return &dto.ResolvedPreferencesResultDTO{Preferences: dtos}, service.ErrNone, nil
}

// CheckRecipientTargetSubscription resolves ONE (target, medium) the way a send
// would decide it.
//
// It used to walk the stored rows in Go and exact-match them, which was wrong in
// the same ways the old list read was: it never consulted the topic='any'
// fallbacks, and its not-found default returned enabled=true for EVERY medium —
// so an email target that would never fire reported as subscribed. It now runs
// the shared cascade.
//
// The target need not be cataloged or stored at all, which is why this resolves
// an explicit target universe rather than filtering the "everything known" read:
// an unknown target still resolves (a project topic='any' rule can cover it, and
// in_app delivers by default).
func (s *PreferenceService) CheckRecipientTargetSubscription(ctx context.Context, projectID int, recipientExtID string, payload dto.CheckRecipientTargetPayload) (*dto.PreferenceTargetResolvedStateDTO, service.Error, error) {
	if err := payload.Validate(); err != nil {
		return nil, service.ErrInvalidInput, err
	}

	target := dto.Target{
		Channel: payload.Channel,
		Topic:   payload.Topic,
		Event:   payload.Event,
	}

	resolved, err := s.repo.ResolveRecipientPreferenceForTargets(
		ctx, projectID, recipientExtID,
		[]enum.Medium{enum.Medium(payload.Medium)},
		[]dto.Target{target},
	)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("repo resolve recipient preference for target: %w", err)
	}

	if len(resolved) != 1 {
		return nil, service.ErrInternalServerError, fmt.Errorf("resolve returned %d cells for one (target, medium)", len(resolved))
	}

	return dto.FromResolvedPreferenceForAPI(resolved[0]), service.ErrNone, nil
}

func (s *PreferenceService) Delete(ctx context.Context, payload *dto.DeletePreferencePayload) (service.Error, error) {
	err := payload.Validate()
	if err != nil {
		return service.ErrInvalidInput, err
	}

	err = s.repo.Delete(ctx, payload.ProjectID, payload.PreferenceID)
	if err != nil {
		if err == tantraRepo.ErrNotFound {
			return service.ErrNotFound, fmt.Errorf("Preference not found")
		}
		return service.ErrInternalServerError, fmt.Errorf("repo delete preference: %w", err)
	}

	return service.ErrNone, nil
}

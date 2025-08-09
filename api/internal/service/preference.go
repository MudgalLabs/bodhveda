package service

import (
	"context"
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
		&payload.Label,
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

func (s *PreferenceService) ListProjectPreferences(ctx context.Context, projectID int) ([]*dto.ProjectPreferenceListItem, service.Error, error) {
	prefs, err := s.repo.ListPreferences(ctx, projectID, enum.PreferenceKindProject)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("repo list preferences: %w", err)
	}

	dtos := []*dto.ProjectPreferenceListItem{}
	for _, pref := range prefs {
		target := dto.TargetFromPreference(pref)

		recipients, err := s.repo.ListEligibleRecipientExtIDsForBroadcast(ctx, projectID, target)
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

	pref := entity.NewPreference(
		&payload.ProjectID,
		&payload.RecipientExtID,
		payload.Channel,
		payload.Topic,
		payload.Event,
		nil,
		payload.Enabled,
	)

	pref.UpdatedAt = time.Now().UTC()

	newPref, err := s.repo.Create(ctx, pref)
	if err != nil {
		if err == tantraRepo.ErrConflict {
			return nil, service.ErrConflict, fmt.Errorf("Preference already exists")
		}
		return nil, service.ErrInternalServerError, fmt.Errorf("repo create preference: %w", err)
	}

	return dto.FromPreferenceForRecipient(newPref), service.ErrNone, nil
}

func (s *PreferenceService) PatchRecipientPreferenceTarget(ctx context.Context, projectID int, recipientExtID string, req dto.PatchRecipientPreferenceTargetPayload) (*dto.PreferenceTargetStateDTO, service.Error, error) {
	if err := req.Validate(); err != nil {
		return nil, service.ErrInvalidInput, err
	}

	// Upsert recipient-level preference for this target
	pref := entity.NewPreference(
		&projectID,
		&recipientExtID,
		req.Target.Channel,
		req.Target.Topic,
		req.Target.Event,
		nil,
		req.State.Subscribed,
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

func (s *PreferenceService) GetRecipientGlobalPreferences(ctx context.Context, projectID int, recipientExtID string) (*dto.PreferenceTargetStatesResultDTO, service.Error, error) {
	// 1. Fetch all project-level preferences (these are the defaults for all recipients)
	projectPrefs, err := s.repo.ListPreferences(ctx, projectID, enum.PreferenceKindProject)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("repo list project preferences: %w", err)
	}

	// 2. Fetch all recipient-level preferences for this recipient (these override project-level preferences)
	recipientPrefs, err := s.repo.ListPreferencesForRecipient(ctx, projectID, recipientExtID)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("repo list recipient preferences: %w", err)
	}

	// 3. Build a map for quick lookup of recipient-level preferences by (channel, topic, event)
	type prefKey struct{ Channel, Topic, Event string }
	recipientPrefMap := make(map[prefKey]*entity.Preference)
	for _, pref := range recipientPrefs {
		// Only consider preferences for the given recipient
		if pref.RecipientExtID != nil && *pref.RecipientExtID == recipientExtID {
			recipientPrefMap[prefKey{pref.Channel, pref.Topic, pref.Event}] = pref
		}
	}

	result := []*dto.PreferenceTargetStateDTO{}
	// 4. For each project-level preference, check if there is a recipient-level override
	for _, projPref := range projectPrefs {
		key := prefKey{projPref.Channel, projPref.Topic, projPref.Event}

		item := dto.PreferenceTargetStateDTO{
			Target: dto.PreferenceTargetDTOFromPreference(projPref),
			State: dto.PreferenceStateDTO{
				Subscribed: projPref.Enabled,
				Inherited:  true,
			},
		}

		if projPref.Label != nil {
			item.Target.Label = projPref.Label
		}

		// 5. If a recipient-level preference exists, override the project-level setting
		if rp, ok := recipientPrefMap[key]; ok {
			item.State.Subscribed = rp.Enabled
			item.State.Inherited = false
		}

		result = append(result, &item)
	}

	// 6. Return the merged preferences (project-level, overridden by recipient-level where applicable)
	return &dto.PreferenceTargetStatesResultDTO{
		GlobalPreferences: result,
	}, service.ErrNone, nil
}

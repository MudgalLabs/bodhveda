package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mudgallabs/bodhveda/internal/job/task"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	tantraRepo "github.com/mudgallabs/tantra/repository"
	"github.com/mudgallabs/tantra/service"
)

type RecipientService struct {
	repo        repository.RecipientRepository
	asynqClient *asynq.Client
}

func NewRecipientService(repo repository.RecipientRepository, asynqClient *asynq.Client) *RecipientService {
	return &RecipientService{
		repo:        repo,
		asynqClient: asynqClient,
	}
}

func (s *RecipientService) Create(ctx context.Context, payload dto.CreateRecipientPayload) (*dto.Recipient, service.Error, error) {
	err := payload.Validate()
	if err != nil {
		return nil, service.ErrInvalidInput, err
	}

	name := ""
	if payload.Name != nil {
		name = *payload.Name
	}

	recipient := entity.NewRecipient(payload.ProjectID, payload.ExternalID, name)
	recipient, err = s.repo.Create(ctx, recipient)
	if err != nil {
		if err == tantraRepo.ErrConflict {
			return nil, service.ErrConflict, fmt.Errorf("Recipient already exists")
		}
		return nil, service.ErrInternalServerError, fmt.Errorf("recipient repo create: %w", err)
	}

	return dto.FromRecipient(recipient), service.ErrNone, nil
}

func (s *RecipientService) CreateIfNotExists(ctx context.Context, payload dto.CreateRecipientPayload) (*dto.Recipient, service.Error, error) {
	err := payload.Validate()
	if err != nil {
		return nil, service.ErrInvalidInput, err
	}

	name := ""
	if payload.Name != nil {
		name = *payload.Name
	}

	recipient := entity.NewRecipient(payload.ProjectID, payload.ExternalID, name)
	recipient, err = s.repo.Create(ctx, recipient)
	if err != nil {
		// If recipient already exists, fetch and return it.
		if err == tantraRepo.ErrConflict {
			recipient, err = s.repo.Get(ctx, payload.ProjectID, payload.ExternalID)
			return dto.FromRecipient(recipient), service.ErrNone, err
		}

		return nil, service.ErrInternalServerError, fmt.Errorf("recipient repo create: %w", err)
	}

	return dto.FromRecipient(recipient), service.ErrNone, nil
}

func (s *RecipientService) List(ctx context.Context, payload *dto.ListRecipientsPayload) (*dto.ListRecipientsResult, service.Error, error) {
	recipients, total, err := s.repo.List(ctx, payload.ProjectID, payload.Pagination)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("recipient repo list: %w", err)
	}

	return &dto.ListRecipientsResult{
		Recipients: dto.FromRecipientList(recipients),
		Pagination: payload.Pagination.GetMeta(total),
	}, service.ErrNone, nil
}

func (s *RecipientService) Get(ctx context.Context, projectID int, externalID string) (*dto.Recipient, service.Error, error) {
	if projectID <= 0 || externalID == "" {
		return nil, service.ErrInvalidInput, fmt.Errorf("projectID and externalID required")
	}

	recipient, err := s.repo.Get(ctx, projectID, externalID)
	if err != nil {
		if err == tantraRepo.ErrNotFound {
			return nil, service.ErrNotFound, err
		}

		return nil, service.ErrInternalServerError, fmt.Errorf("recipient repo get: %w", err)
	}

	return dto.FromRecipient(recipient), service.ErrNone, nil
}

func (s *RecipientService) Update(ctx context.Context, projectID int, externalID string, payload *dto.UpdateRecipientPayload) (*dto.Recipient, service.Error, error) {
	if externalID == "" {
		return nil, service.ErrInvalidInput, fmt.Errorf("recipient id required")
	}

	if err := payload.Validate(); err != nil {
		return nil, service.ErrInvalidInput, err
	}

	recipient, err := s.repo.Update(ctx, projectID, externalID, payload)
	if err != nil {
		if err == tantraRepo.ErrNotFound {
			return nil, service.ErrNotFound, err
		}

		return nil, service.ErrInternalServerError, err
	}

	return dto.FromRecipient(recipient), service.ErrNone, nil
}

func (s *RecipientService) Delete(ctx context.Context, projectID int, externalID string) (service.Error, error) {
	if externalID == "" {
		return service.ErrInvalidInput, fmt.Errorf("recipient id required")
	}

	err := s.repo.SoftDelete(ctx, projectID, externalID)
	if err != nil {
		if err == tantraRepo.ErrNotFound {
			return service.ErrNotFound, fmt.Errorf("Recipient not found")
		}
		return service.ErrInternalServerError, err
	}

	taskPayload := dto.DeleteRecipientDataPayload{
		ProjectID:      projectID,
		RecipientExtID: externalID,
	}

	payload, err := json.Marshal(taskPayload)
	if err != nil {
		return service.ErrInternalServerError, fmt.Errorf("marshal delete recipient data payload: %w", err)
	}

	task := asynq.NewTask(task.TaskTypeDeleteRecipientData, payload)
	_, err = s.asynqClient.Enqueue(task, asynq.MaxRetry(3))
	if err != nil {
		return service.ErrInternalServerError, fmt.Errorf("enqueue delete recipient data task: %w", err)
	}

	return service.ErrNone, nil
}

func (s *RecipientService) BatchCreate(ctx context.Context, payloads []dto.CreateRecipientPayload) (*dto.BatchCreateRecipientsResult, service.Error, error) {
	if len(payloads) == 0 {
		return nil, service.ErrInvalidInput, fmt.Errorf("no recipients provided")
	}

	if len(payloads) > 1000 {
		return nil, service.ErrInvalidInput, fmt.Errorf("batch size exceeds limit of 1000")
	}

	var (
		recipients = []*entity.Recipient{}
		created    = []dto.BatchCreateRecipientCreated{}
		updated    = []dto.BatchCreateRecipientUpdated{}
		failed     = []dto.BatchCreateRecipientFailed{}
	)

	for i, p := range payloads {
		if err := p.Validate(); err != nil {
			failed = append(failed, dto.BatchCreateRecipientFailed{
				Errors:         err.(service.InputValidationErrors),
				RecipientExtID: p.ExternalID,
				BatchIndex:     i,
			})
			continue
		}
		name := ""
		if p.Name != nil {
			name = *p.Name
		}
		recipients = append(recipients, entity.NewRecipient(p.ProjectID, p.ExternalID, name))
	}

	createdIDs, updatedIDs, err := s.repo.BatchCreate(ctx, recipients)
	if err != nil {
		return nil, service.ErrInternalServerError, err
	}

	for _, id := range createdIDs {
		created = append(created, dto.BatchCreateRecipientCreated{
			RecipientExtID: id,
		})
	}
	for _, id := range updatedIDs {
		updated = append(updated, dto.BatchCreateRecipientUpdated{
			RecipientExtID: id,
		})
	}

	result := &dto.BatchCreateRecipientsResult{
		Created: created,
		Updated: updated,
		Failed:  failed,
	}

	return result, service.ErrNone, nil
}

func (s *RecipientService) CreateRandomRecipients(ctx context.Context, projectID int, count int) error {
	names := []string{
		"Alice Johnson", "Bob Smith", "Charlie Brown", "Diana Prince", "Edward Norton",
		"Fiona Green", "George Wilson", "Hannah Lee", "Ian Davis", "Julia Roberts",
		"Kevin Hart", "Linda Chen", "Michael Scott", "Nancy Drew", "Oliver Twist",
		"Patricia Moore", "Quinn Adams", "Rachel Green", "Steven King", "Tina Turner",
		"Uma Thurman", "Victor Hugo", "Wendy Williams", "Xavier Woods", "Yvonne Carter",
		"Zachary Taylor", "Amanda Clarke", "Benjamin Franklin", "Catherine Zeta", "Daniel Craig",
	}

	recipients := make([]*entity.Recipient, count)
	now := time.Now().UTC()

	for i := range recipients {
		externalID := fmt.Sprintf("user_%d_%d", projectID, 10+i) // Unique external ID for each recipient

		recipients[i] = &entity.Recipient{
			ExternalID: externalID,
			ProjectID:  projectID,
			Name:       names[rand.Intn(len(names))],
			CreatedAt:  now,
			UpdatedAt:  now,
		}
	}

	_, _, err := s.repo.BatchCreate(ctx, recipients)
	return err
}

func (s *RecipientService) TotalCount(ctx context.Context, projectID int) (int, service.Error, error) {
	if projectID <= 0 {
		return 0, service.ErrInvalidInput, fmt.Errorf("projectID required")
	}

	count, err := s.repo.TotalCount(ctx, projectID)
	if err != nil {
		return 0, service.ErrInternalServerError, fmt.Errorf("recipient repo total count: %w", err)
	}

	return count, service.ErrNone, nil
}

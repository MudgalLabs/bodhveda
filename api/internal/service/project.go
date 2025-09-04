package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/mudgallabs/bodhveda/internal/job/task"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	tantraRepo "github.com/mudgallabs/tantra/repository"
	"github.com/mudgallabs/tantra/service"
)

type ProjectService struct {
	repo repository.ProjectRepository

	notificationService *NotificationService
	recipientService    *RecipientService

	asynqClient *asynq.Client
}

func NewProjectService(
	repo repository.ProjectRepository,
	notificationService *NotificationService, recipientService *RecipientService,
	asynqClient *asynq.Client,
) *ProjectService {
	return &ProjectService{
		repo,
		notificationService,
		recipientService,
		asynqClient,
	}
}

func (s *ProjectService) Create(ctx context.Context, payload dto.CreateProjectPaylaod) (*dto.Project, service.Error, error) {
	// TODO: Limit free users to 1/2 project.

	err := payload.Validate()
	if err != nil {
		return nil, service.ErrInvalidInput, err
	}

	project := entity.NewProject(payload.UserID, payload.Name)
	project, err = s.repo.Create(ctx, project)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("project repo create: %w", err)
	}

	return dto.FromProject(project), service.ErrNone, nil
}

func (s *ProjectService) List(ctx context.Context, userID int) ([]*dto.ProjectListItem, service.Error, error) {
	projects, err := s.repo.List(ctx, userID)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("project repo list: %w", err)
	}

	list := []*dto.ProjectListItem{}

	for _, project := range projects {
		listItem := dto.ProjectListItem{
			Project: dto.FromProject(project),
		}

		overviewResult, _, err := s.notificationService.Overview(ctx, project.ID)
		if err != nil {
			return nil, service.ErrInternalServerError, fmt.Errorf("get notifications overview: %w", err)
		}
		listItem.NotificationsOverviewResult = overviewResult

		recipientCount, _, err := s.recipientService.TotalCount(ctx, project.ID)
		if err != nil {
			return nil, service.ErrInternalServerError, fmt.Errorf("get recipients count: %w", err)
		}
		listItem.TotalRecipientsCount = recipientCount

		list = append(list, &listItem)
	}

	return list, service.ErrNone, nil
}

func (s *ProjectService) Delete(ctx context.Context, userID, projectID int) (service.Error, error) {
	err := s.repo.SoftDelete(ctx, userID, projectID)
	if err != nil {
		if err == tantraRepo.ErrNotFound {
			return service.ErrNotFound, nil
		}
		return service.ErrInternalServerError, fmt.Errorf("project repo soft delete: %w", err)
	}

	data := dto.DeleteProjectDataPayload{
		ProjectID: projectID,
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return service.ErrInternalServerError, fmt.Errorf("marshal delete project data payload: %w", err)
	}

	task := asynq.NewTask(task.TaskTypeDeleteProjectData, payload)
	_, err = s.asynqClient.Enqueue(task, asynq.MaxRetry(3))
	if err != nil {
		return service.ErrInternalServerError, fmt.Errorf("enqueue delete project data task: %w", err)
	}

	return service.ErrNone, nil
}

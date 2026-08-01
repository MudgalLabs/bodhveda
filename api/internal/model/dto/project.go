package dto

import (
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/tantra/apires"
	"github.com/mudgallabs/tantra/service"
)

type Project struct {
	ID   int    `json:"id"`
	Name string `json:"name"`

	// StrictTargets reports whether this project rejects sends to targets it has
	// not cataloged. False by default — see entity.Project.
	StrictTargets bool `json:"strict_targets"`
}

type CreateProjectPaylaod struct {
	UserID int
	Name   string `json:"name"`
}

func (p *CreateProjectPaylaod) Validate() error {
	var errs service.InputValidationErrors

	if p.UserID <= 0 {
		errs.Add(apires.NewApiError("User is required", "User ID must be a positive integer", "user_id", p.UserID))
	}

	if p.Name == "" {
		errs.Add(apires.NewApiError("Name is required", "Name cannot be empty", "name", p.Name))
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

type UpdateProjectPayload struct {
	UserID    int
	ProjectID int
	Name      string `json:"name"`

	// StrictTargets is a POINTER so that omitting it means "leave it alone"
	// rather than "turn it off". The console's rename dialog sends only `name`;
	// with a plain bool that request would silently disable the gate on a project
	// that had deliberately enabled it.
	StrictTargets *bool `json:"strict_targets"`
}

func (p *UpdateProjectPayload) Validate() error {
	var errs service.InputValidationErrors

	if p.UserID <= 0 {
		errs.Add(apires.NewApiError("User is required", "User ID must be a positive integer", "user_id", p.UserID))
	}

	if p.ProjectID <= 0 {
		errs.Add(apires.NewApiError("Project is required", "Project ID must be a positive integer", "project_id", p.ProjectID))
	}

	if p.Name == "" {
		errs.Add(apires.NewApiError("Name is required", "Name cannot be empty", "name", p.Name))
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func FromProject(p *entity.Project) *Project {
	if p == nil {
		return nil
	}

	return &Project{
		ID:            p.ID,
		Name:          p.Name,
		StrictTargets: p.StrictTargets,
	}
}

func FromProjects(p []*entity.Project) []*Project {
	if p == nil {
		return nil
	}

	dtos := make([]*Project, len(p))
	for i, project := range p {
		dtos[i] = FromProject(project)
	}

	return dtos
}

type ProjectListItem struct {
	*Project
	*NotificationsOverviewResult

	TotalRecipientsCount int `json:"total_recipients"`
}

type DeleteProjectDataPayload struct {
	ProjectID int `json:"project_id"`
}

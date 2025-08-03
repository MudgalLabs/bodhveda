package dto

import (
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/tantra/apires"
	"github.com/mudgallabs/tantra/service"
)

type Project struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
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

func FromProject(p *entity.Project) *Project {
	if p == nil {
		return nil
	}

	return &Project{
		ID:   p.ID,
		Name: p.Name,
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

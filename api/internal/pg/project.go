package pg

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
)

type ProjectRepo struct {
	db   dbx.DBExecutor
	pool *pgxpool.Pool
}

func NewProjectRepo(db *pgxpool.Pool) repository.ProjectRepository {
	return &ProjectRepo{
		db:   db,
		pool: db,
	}
}

func (r *ProjectRepo) Create(ctx context.Context, project *entity.Project) (*entity.Project, error) {
	sql := `
		INSERT INTO project (user_id, name, created_at, updated_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, name, created_at, updated_at
	`
	row := r.db.QueryRow(ctx, sql, project.UserID, project.Name, project.CreatedAt, project.UpdatedAt)

	var p entity.Project

	err := row.Scan(&p.ID, &p.UserID, &p.Name, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &p, nil
}

func (r *ProjectRepo) List(ctx context.Context, userID int) ([]*entity.Project, error) {
	sql := `
		SELECT id, user_id, name, created_at, updated_at
		FROM project
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, sql, userID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	projects := []*entity.Project{}
	for rows.Next() {
		var p entity.Project
		err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, err
		}
		projects = append(projects, &p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return projects, nil
}

func (r *ProjectRepo) UserOwns(ctx context.Context, userID, projectID int) (bool, error) {
	sql := `
		SELECT EXISTS (
			SELECT 1 FROM project
			WHERE user_id = $1 AND id = $2
		)
	`

	var exists bool

	err := r.db.QueryRow(ctx, sql, userID, projectID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

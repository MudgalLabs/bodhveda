package pg

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
	tantraRepo "github.com/mudgallabs/tantra/repository"
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

// projectColumns is the one place the projection is written. Every read below
// scans it in this order via scanProject.
const projectColumns = `id, user_id, name, strict_targets, created_at, updated_at`

type scannable interface {
	Scan(dest ...any) error
}

func scanProject(row scannable) (*entity.Project, error) {
	var p entity.Project
	err := row.Scan(&p.ID, &p.UserID, &p.Name, &p.StrictTargets, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *ProjectRepo) Create(ctx context.Context, project *entity.Project) (*entity.Project, error) {
	sql := `
		INSERT INTO project (user_id, name, strict_targets, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING ` + projectColumns
	row := r.db.QueryRow(ctx, sql, project.UserID, project.Name, project.StrictTargets, project.CreatedAt, project.UpdatedAt)

	return scanProject(row)
}

func (r *ProjectRepo) Get(ctx context.Context, projectID int) (*entity.Project, error) {
	sql := `
		SELECT ` + projectColumns + `
		FROM project
		WHERE id = $1 AND deleted_at IS NULL
	`

	p, err := scanProject(r.db.QueryRow(ctx, sql, projectID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, tantraRepo.ErrNotFound
		}
		return nil, err
	}

	return p, nil
}

func (r *ProjectRepo) List(ctx context.Context, userID int) ([]*entity.Project, error) {
	sql := `
		SELECT ` + projectColumns + `
		FROM project
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, sql, userID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	projects := []*entity.Project{}
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return projects, nil
}

// Update is a PARTIAL update: a nil field means "not supplied", and the stored
// value is kept. Writing the zero value instead would let a rename silently turn
// strict targets off — the same bug that had to be fixed for preference.mandatory
// (COALESCE below is the same remedy).
func (r *ProjectRepo) Update(ctx context.Context, userID, projectID int, name string, strictTargets *bool) (*entity.Project, error) {
	sql := `
		UPDATE project
		SET name = $1,
		    strict_targets = COALESCE($2, strict_targets),
		    updated_at = $3
		WHERE user_id = $4 AND id = $5 AND deleted_at IS NULL
		RETURNING ` + projectColumns
	row := r.db.QueryRow(ctx, sql, name, strictTargets, time.Now().UTC(), userID, projectID)

	p, err := scanProject(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, tantraRepo.ErrNotFound
		}
		return nil, err
	}

	return p, nil
}

func (r *ProjectRepo) UserOwns(ctx context.Context, userID, projectID int) (bool, error) {
	sql := `
		SELECT EXISTS (
			SELECT 1 FROM project
			WHERE user_id = $1 AND id = $2 AND deleted_at IS NULL
		)
	`

	var exists bool

	err := r.db.QueryRow(ctx, sql, userID, projectID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (r *ProjectRepo) SoftDelete(ctx context.Context, userID, projectID int) error {
	sql := `
		UPDATE project SET deleted_at = $1
		WHERE user_id = $2 AND id = $3
	`
	tag, err := r.db.Exec(ctx, sql, time.Now().UTC(), userID, projectID)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return tantraRepo.ErrNotFound
	}

	return err
}

func (r *ProjectRepo) Delete(ctx context.Context, projectID int) error {
	sql := `
		DELETE FROM project WHERE id = $1
	`
	_, err := r.db.Exec(ctx, sql, projectID)
	return err
}

package user_profile

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/tantra/dbx"
	"github.com/mudgallabs/tantra/repository"
)

type Reader interface {
	FindUserProfileByUserID(ctx context.Context, userID int) (*UserProfile, error)
}

type Writer interface {
	Update(ctx context.Context, userProfile *UserProfile) error
}

type ReadWriter interface {
	Reader
	Writer
}

//
// PostgreSQL implementation
//

type filter struct {
	UserID *int
	Email  *string
}

type userProfileRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *userProfileRepository {
	return &userProfileRepository{db}
}

func (r *userProfileRepository) FindUserProfileByUserID(ctx context.Context, userID int) (*UserProfile, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}

	defer tx.Rollback(ctx)

	userProfiles, err := r.findUserProfiles(ctx, tx, &filter{UserID: &userID})
	if err != nil {
		return nil, fmt.Errorf("find user profiles: %w", err)
	}

	if len(userProfiles) == 0 {
		return nil, repository.ErrNotFound
	}

	userProfile := userProfiles[0]
	return userProfile, nil
}

func (r *userProfileRepository) findUserProfiles(ctx context.Context, tx pgx.Tx, f *filter) ([]*UserProfile, error) {
	baseSQL := `
	SELECT user_id, email, name, avatar_url, created_at, updated_at
	FROM user_profile`
	b := dbx.NewSQLBuilder(baseSQL)

	if v := f.UserID; v != nil {
		b.AddCompareFilter("user_id", "=", *v)
	}

	if v := f.Email; v != nil {
		b.AddCompareFilter("email", "=", *v)
	}

	query, args := b.Build()

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	defer rows.Close()

	var userProfiles []*UserProfile
	for rows.Next() {
		var up UserProfile

		err := rows.Scan(&up.UserID, &up.Email, &up.Name, &up.AvatarURL, &up.CreatedAt, &up.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}

		userProfiles = append(userProfiles, &up)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}

	return userProfiles, nil
}

func (r *userProfileRepository) Update(ctx context.Context, userProfile *UserProfile) error {
	updatedAt := time.Now().UTC()

	sql := `
		UPDATE user_profile
		SET email = $1, name = $2, avatar_url = $3, updated_at = $4
		WHERE user_id = $5
	`

	_, err := r.db.Exec(ctx, sql, userProfile.Email, userProfile.Name, userProfile.AvatarURL, updatedAt, userProfile.UserID)
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}

	return nil
}

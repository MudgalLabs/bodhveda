package user_profile

import (
	"time"
)

type UserProfile struct {
	UserID    int       `json:"user_id" db:"user_id"`
	Email     string    `json:"email" db:"email"`
	Name      string    `json:"name" db:"name"`
	AvatarURL string    `json:"avatar_url" db:"avatar_url"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

func NewUserProfile(userID int, email, name string) *UserProfile {
	now := time.Now().UTC()
	return &UserProfile{
		UserID:    userID,
		Email:     email,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

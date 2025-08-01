package entity

import (
	"time"
)

type Project struct {
	ID        int
	Name      string
	UserID    int
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewProject(userID int, name string) {}

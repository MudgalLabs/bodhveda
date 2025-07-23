// Package project provides services related to project management.
package project

import "github.com/google/uuid"

type Service struct {
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) GetProjectFromAPIKey(key string) (uuid.UUID, error) {
	// Fetch API Key from database. Then get the ProjectID associated with it.
	// This is a placeholder implementation.
	testProjectID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	return testProjectID, nil
}

package jobs

import (
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/mudgallabs/bodhveda/internal/env"
)

// NewAsynqClient initializes and returns a new Asynq client.
// Client is used to enqueue tasks to the Asynq server.
func NewAsynqClient() (*asynq.Client, error) {
	redisConnOpt, err := asynq.ParseRedisURI(env.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	return asynq.NewClient(redisConnOpt), nil
}

// NewAsynqServer initializes and returns a new Asynq server.
// Server is used to process tasks from the Asynq queue.
func NewAsynqServer() (*asynq.Server, error) {
	redisConnOpt, err := asynq.ParseRedisURI(env.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	return asynq.NewServer(
		redisConnOpt,
		asynq.Config{
			// Specify how many concurrent workers to use
			Concurrency: 10,
		},
	), nil
}

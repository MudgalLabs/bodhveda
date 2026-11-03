package job

import (
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/mudgallabs/bodhveda/internal/env"
)

// Concurrency is how many tasks the worker runs at once. app sizes the DB pool
// from it (see app.DBPoolMaxConns): two connections per concurrent task.
const Concurrency = 10

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
			Concurrency: Concurrency,
		},
	), nil
}

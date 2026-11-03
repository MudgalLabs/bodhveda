package app

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	jobs "github.com/mudgallabs/bodhveda/internal/job"
)

func TestWithPoolMaxConns(t *testing.T) {
	cases := []struct {
		name, in string
		want     int32
	}{
		{"adds the floor", "postgres://u:p@db:5432/bv?sslmode=disable", 20},
		{"no query string", "postgresql://u:p@db/bv", 20},
		{"operator value wins", "postgres://u:p@db/bv?pool_max_conns=7&sslmode=disable", 7},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg, err := pgxpool.ParseConfig(withPoolMaxConns(c.in, 20))
			if err != nil {
				t.Fatalf("pgxpool rejected the rewritten URL: %v", err)
			}
			if cfg.MaxConns != c.want {
				t.Errorf("MaxConns = %d, want %d", cfg.MaxConns, c.want)
			}
		})
	}

	if got := withPoolMaxConns("host=db dbname=bv", 20); got != "host=db dbname=bv" {
		t.Errorf("a key/value DSN must pass through untouched, got %q", got)
	}
}

func TestPoolFloorCoversWorkerConcurrency(t *testing.T) {
	// Two connections per concurrent task is what makes the pool impossible to
	// exhaust with tx-holders each waiting for one more. See DBPoolMaxConns.
	if DBPoolMaxConns < 2*jobs.Concurrency {
		t.Fatalf("DBPoolMaxConns = %d, below 2x worker concurrency", DBPoolMaxConns)
	}
}

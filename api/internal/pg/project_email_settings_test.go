package pg

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/env"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	tantraRepo "github.com/mudgallabs/tantra/repository"
)

// TestProjectEmailSettingsRepo_RoundTrip exercises the real upsert/get/conflict
// SQL against a live Postgres. It is skipped unless TEST_DB_URL is set (and needs
// an existing project row id in TEST_PROJECT_ID, default 1). Cleans up after
// itself by deleting the row it wrote.
func TestProjectEmailSettingsRepo_RoundTrip(t *testing.T) {
	dbURL := os.Getenv("TEST_DB_URL")
	if dbURL == "" {
		t.Skip("TEST_DB_URL not set; skipping DB integration test")
	}

	// Any valid AES-256 key — the repo stores opaque bytes, encryption is the
	// entity's job.
	env.CipherKey = "0123456789abcdef0123456789abcdef"

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	const projectID = 1

	repo := NewProjectEmailSettingsRepo(pool)
	// Cleanup runs LIFO: delete the row first (pool still open), then close.
	t.Cleanup(pool.Close)
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, "DELETE FROM project_email_settings WHERE project_id = $1", projectID); err != nil {
			t.Errorf("cleanup delete failed: %v", err)
		}
	})

	// Not configured yet → ErrNotFound.
	if _, err := repo.Get(ctx, projectID); err != tantraRepo.ErrNotFound {
		t.Fatalf("Get before insert: got %v, want ErrNotFound", err)
	}

	// Insert.
	s1, err := entity.NewProjectEmailSettings(projectID, enum.EmailProviderResend, "re_key_one", "Acme", "hey@acme.com")
	if err != nil {
		t.Fatalf("NewProjectEmailSettings: %v", err)
	}
	saved, err := repo.Upsert(ctx, s1)
	if err != nil {
		t.Fatalf("Upsert insert: %v", err)
	}
	if saved.FromName != "Acme" || saved.Provider != enum.EmailProviderResend {
		t.Fatalf("insert returned wrong row: %+v", saved)
	}
	got, err := saved.DecryptSecret()
	if err != nil || got != "re_key_one" {
		t.Fatalf("decrypt after insert: got %q err %v", got, err)
	}

	// Upsert again (ON CONFLICT path) with a rotated key + new identity.
	s2, err := entity.NewProjectEmailSettings(projectID, enum.EmailProviderResend, "re_key_two", "Acme Inc", "team@acme.com")
	if err != nil {
		t.Fatalf("NewProjectEmailSettings 2: %v", err)
	}
	s2.CreatedAt = saved.CreatedAt // preserve created_at like the service does
	updated, err := repo.Upsert(ctx, s2)
	if err != nil {
		t.Fatalf("Upsert conflict update: %v", err)
	}
	if updated.FromName != "Acme Inc" || updated.FromAddress != "team@acme.com" {
		t.Fatalf("update did not apply: %+v", updated)
	}
	got2, err := updated.DecryptSecret()
	if err != nil || got2 != "re_key_two" {
		t.Fatalf("decrypt after update: got %q err %v", got2, err)
	}

	// Get returns the updated row.
	fetched, err := repo.Get(ctx, projectID)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if fetched.FromName != "Acme Inc" {
		t.Fatalf("Get returned stale row: %+v", fetched)
	}
}

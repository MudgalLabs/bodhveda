package pg

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	tantraRepo "github.com/mudgallabs/tantra/repository"
)

// TestPreferenceCatalogCRUD exercises the Developer-API catalog CRUD repo
// methods (GetProjectPreferenceByID / UpdateProjectPreference /
// DeleteProjectPreference) against a live Postgres. The load-bearing assertion
// is SCOPING: all three are confined to project-level rows (recipient NULL), so
// a full-scope key driving the catalog surface can neither read nor delete a
// recipient's own preference row by id.
//
// Skipped unless TEST_DB_URL is set. Self-cleaning.
func TestPreferenceCatalogCRUD(t *testing.T) {
	dbURL := os.Getenv("TEST_DB_URL")
	if dbURL == "" {
		t.Skip("TEST_DB_URL not set; skipping DB integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	extID := "catalog-crud-user"

	var userID int
	if err := pool.QueryRow(ctx, `SELECT user_id FROM project ORDER BY id LIMIT 1`).Scan(&userID); err != nil {
		t.Fatalf("need at least one existing project to borrow a user_id: %v", err)
	}

	var projectID int
	err = pool.QueryRow(ctx, `
		INSERT INTO project (user_id, name, created_at, updated_at)
		VALUES ($1, 'catalog-crud-test', now(), now()) RETURNING id
	`, userID).Scan(&projectID)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DELETE FROM project WHERE id = $1", projectID) })

	_, err = pool.Exec(ctx, `
		INSERT INTO recipient (external_id, name, project_id, created_at, updated_at)
		VALUES ($1, 'Catalog CRUD', $2, now(), now())
	`, extID, projectID)
	if err != nil {
		t.Fatalf("insert recipient: %v", err)
	}

	repo := NewPreferenceRepo(pool)

	// Seed a catalog entry (project-level row) via the same Create the create
	// endpoint uses.
	label := "Daily digest"
	created, err := repo.Create(ctx, entity.NewPreference(
		&projectID, nil, "digest", "none", "sent", "email", &label, true,
	))
	if err != nil {
		t.Fatalf("Create catalog entry: %v", err)
	}

	// And a recipient-level row for the SAME target/medium — this is what the
	// scoping assertions below must never touch.
	recipientRow, err := repo.Create(ctx, entity.NewPreference(
		&projectID, &extID, "digest", "none", "sent", "email", nil, false,
	))
	if err != nil {
		t.Fatalf("Create recipient row: %v", err)
	}

	t.Run("GetProjectPreferenceByID returns the catalog entry", func(t *testing.T) {
		got, err := repo.GetProjectPreferenceByID(ctx, projectID, created.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.Label == nil || *got.Label != "Daily digest" || got.Medium != "email" || !got.Enabled {
			t.Fatalf("wrong row: %+v", got)
		}
	})

	t.Run("GetProjectPreferenceByID 404s for an unknown id", func(t *testing.T) {
		if _, err := repo.GetProjectPreferenceByID(ctx, projectID, 999999); err != tantraRepo.ErrNotFound {
			t.Fatalf("got %v, want ErrNotFound", err)
		}
	})

	t.Run("GetProjectPreferenceByID 404s for a recipient-level row's id", func(t *testing.T) {
		if _, err := repo.GetProjectPreferenceByID(ctx, projectID, recipientRow.ID); err != tantraRepo.ErrNotFound {
			t.Fatalf("catalog get reached a recipient row: got %v, want ErrNotFound", err)
		}
	})

	t.Run("UpdateProjectPreference changes label + default and returns the row", func(t *testing.T) {
		updated, err := repo.UpdateProjectPreference(ctx, projectID, created.ID, "Weekly digest", false)
		if err != nil {
			t.Fatalf("update: %v", err)
		}
		if updated.Label == nil || *updated.Label != "Weekly digest" || updated.Enabled {
			t.Fatalf("update did not apply: %+v", updated)
		}
		// Natural key is unchanged.
		if updated.Channel != "digest" || updated.Topic != "none" || updated.Event != "sent" || updated.Medium != "email" {
			t.Fatalf("update mutated the natural key: %+v", updated)
		}
	})

	t.Run("UpdateProjectPreference 404s for a recipient-level row's id", func(t *testing.T) {
		if _, err := repo.UpdateProjectPreference(ctx, projectID, recipientRow.ID, "x", true); err != tantraRepo.ErrNotFound {
			t.Fatalf("catalog update reached a recipient row: got %v, want ErrNotFound", err)
		}
	})

	t.Run("DeleteProjectPreference 404s for a recipient-level row's id", func(t *testing.T) {
		if err := repo.DeleteProjectPreference(ctx, projectID, recipientRow.ID); err != tantraRepo.ErrNotFound {
			t.Fatalf("catalog delete reached a recipient row: got %v, want ErrNotFound", err)
		}
		// The recipient row still exists.
		if _, err := repo.GetProjectPreferenceByID(ctx, projectID, recipientRow.ID); err != tantraRepo.ErrNotFound {
			t.Fatalf("unexpected: %v", err)
		}
	})

	t.Run("DeleteProjectPreference removes the catalog entry; second delete 404s", func(t *testing.T) {
		if err := repo.DeleteProjectPreference(ctx, projectID, created.ID); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if err := repo.DeleteProjectPreference(ctx, projectID, created.ID); err != tantraRepo.ErrNotFound {
			t.Fatalf("second delete: got %v, want ErrNotFound", err)
		}
	})
}

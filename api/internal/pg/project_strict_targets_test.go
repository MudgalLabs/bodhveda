package pg

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
)

// projectFixture returns a scratch project owned by a borrowed user id, plus the
// repo under test. Self-cleaning.
func projectFixture(t *testing.T) (context.Context, *pgxpool.Pool, int, int, *ProjectRepo) {
	t.Helper()

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

	var userID int
	if err := pool.QueryRow(ctx, `SELECT user_id FROM project ORDER BY id LIMIT 1`).Scan(&userID); err != nil {
		t.Fatalf("need at least one existing project to borrow a user_id: %v", err)
	}

	repo := NewProjectRepo(pool).(*ProjectRepo)

	project, err := repo.Create(ctx, entity.NewProject(userID, "strict-targets-test"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM project WHERE id = $1`, project.ID)
	})

	return ctx, pool, userID, project.ID, repo
}

// TestNewProjectsDefaultToPermissive pins the product decision, not just the
// column default. An earlier design had NEW projects start strict and only
// grandfather existing ones — which leaves the wall standing for exactly the
// users who cannot see over it, since their first targeted send is the one that
// fails.
func TestNewProjectsDefaultToPermissive(t *testing.T) {
	ctx, _, _, projectID, repo := projectFixture(t)

	project, err := repo.Get(ctx, projectID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if project.StrictTargets {
		t.Fatal("a new project must start permissive; strictness is a maturity setting, opted into once the catalog is stable")
	}
}

// TestUpdateWithoutStrictTargetsKeepsIt is the regression this repo has already
// paid for once: a PATCH that omits a boolean must not write false. The console's
// rename dialog sends only `name`, so a non-partial update would silently switch
// the gate off on a project that deliberately turned it on — the failure mode
// being that sends start succeeding, which nobody notices.
func TestUpdateWithoutStrictTargetsKeepsIt(t *testing.T) {
	ctx, _, userID, projectID, repo := projectFixture(t)

	on := true
	if _, err := repo.Update(ctx, userID, projectID, "strict-targets-test", &on); err != nil {
		t.Fatalf("enable strict targets: %v", err)
	}

	// A rename, carrying no strict_targets at all.
	renamed, err := repo.Update(ctx, userID, projectID, "renamed", nil)
	if err != nil {
		t.Fatalf("rename: %v", err)
	}

	if !renamed.StrictTargets {
		t.Fatal("a rename cleared strict_targets; an omitted field must mean 'unchanged', not 'false'")
	}
	if renamed.Name != "renamed" {
		t.Errorf("name = %q, want %q", renamed.Name, "renamed")
	}
}

// TestUpdateCanTurnStrictTargetsOff — partial update must not become a one-way
// door. Turning it back off is the escape hatch when a project discovers a
// target it forgot to catalog.
func TestUpdateCanTurnStrictTargetsOff(t *testing.T) {
	ctx, _, userID, projectID, repo := projectFixture(t)

	on, off := true, false

	if _, err := repo.Update(ctx, userID, projectID, "p", &on); err != nil {
		t.Fatalf("enable: %v", err)
	}

	updated, err := repo.Update(ctx, userID, projectID, "p", &off)
	if err != nil {
		t.Fatalf("disable: %v", err)
	}

	if updated.StrictTargets {
		t.Fatal("explicit false must turn strict targets off")
	}
}

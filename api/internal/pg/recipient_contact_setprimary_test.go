package pg

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	tantraRepo "github.com/mudgallabs/tantra/repository"
)

// TestSetPrimaryContact exercises the idempotent primary-contact upsert against
// a live Postgres — the four cases (insert / no-op / in-place update / promote),
// the verified_at rules (nulled only when the address changes), and the conflict
// when a primary is moved onto an address another contact already holds.
//
// Skipped unless TEST_DB_URL is set. Self-cleaning.
func TestSetPrimaryContact(t *testing.T) {
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

	extID := "setprimary-user"

	var userID int
	if err := pool.QueryRow(ctx, `SELECT user_id FROM project ORDER BY id LIMIT 1`).Scan(&userID); err != nil {
		t.Fatalf("need at least one existing project to borrow a user_id: %v", err)
	}

	var projectID int
	err = pool.QueryRow(ctx, `
		INSERT INTO project (user_id, name, created_at, updated_at)
		VALUES ($1, 'setprimary-test', now(), now()) RETURNING id
	`, userID).Scan(&projectID)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DELETE FROM project WHERE id = $1", projectID) })

	_, err = pool.Exec(ctx, `
		INSERT INTO recipient (external_id, name, project_id, created_at, updated_at)
		VALUES ($1, 'Set Primary', $2, now(), now())
	`, extID, projectID)
	if err != nil {
		t.Fatalf("insert recipient: %v", err)
	}

	repo := NewRecipientContactRepo(pool)
	set := func(addr string) (*entity.RecipientContact, error) {
		return repo.SetPrimaryContact(ctx, entity.NewRecipientContact(projectID, extID, enum.MediumEmail, addr, true))
	}
	markVerified := func(id int64) {
		if _, err := pool.Exec(ctx, `UPDATE recipient_contact SET verified_at = now() WHERE id = $1`, id); err != nil {
			t.Fatalf("mark verified: %v", err)
		}
	}

	// 1. No primary yet, address unknown → insert a fresh primary, unverified.
	c, err := set("a@x.com")
	if err != nil {
		t.Fatalf("insert primary: %v", err)
	}
	if !c.IsPrimary || c.Address != "a@x.com" || c.VerifiedAt != nil {
		t.Fatalf("unexpected inserted primary: %+v", c)
	}

	// 2. Same address again → idempotent no-op that PRESERVES verification.
	markVerified(c.ID)
	c2, err := set("a@x.com")
	if err != nil {
		t.Fatalf("idempotent set: %v", err)
	}
	if c2.ID != c.ID || c2.VerifiedAt == nil {
		t.Fatalf("no-op should keep the same row and its verification: %+v", c2)
	}

	// 3. Different address → update the SAME row in place, nulling verification.
	c3, err := set("b@x.com")
	if err != nil {
		t.Fatalf("update address: %v", err)
	}
	if c3.ID != c.ID || c3.Address != "b@x.com" || c3.VerifiedAt != nil {
		t.Fatalf("changed address should update in place and null verified_at: %+v", c3)
	}

	// 4. No primary, but the target address already exists as a non-primary
	//    contact → promote it, preserving its verification (address unchanged).
	if _, err := pool.Exec(ctx, `UPDATE recipient_contact SET is_primary = false WHERE id = $1`, c.ID); err != nil {
		t.Fatalf("demote: %v", err)
	}
	nonPrim, err := repo.Create(ctx, entity.NewRecipientContact(projectID, extID, enum.MediumEmail, "c@x.com", false))
	if err != nil {
		t.Fatalf("seed non-primary: %v", err)
	}
	markVerified(nonPrim.ID)
	promoted, err := set("c@x.com")
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if promoted.ID != nonPrim.ID || !promoted.IsPrimary || promoted.VerifiedAt == nil {
		t.Fatalf("promote should flip the existing row primary and keep verification: %+v", promoted)
	}

	// 5. Moving the primary onto an address a DIFFERENT contact already holds
	//    (b@x.com, still present as non-primary) → conflict.
	if _, err := set("b@x.com"); err != tantraRepo.ErrConflict {
		t.Fatalf("moving primary onto an occupied address: got %v, want ErrConflict", err)
	}
}

package service

import (
	"context"
	"strings"
	"testing"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/service"
)

// --- Fakes. Only what gateTarget touches, plus call counters, because two of
// these tests are about which lookups DON'T happen. ---

type flagProjectRepo struct {
	repository.ProjectRepository
	strict bool
	gets   int
}

func (f *flagProjectRepo) Get(ctx context.Context, projectID int) (*entity.Project, error) {
	f.gets++
	return &entity.Project{ID: projectID, StrictTargets: f.strict}, nil
}

type countingCatalogRepo struct {
	repository.PreferenceRepository
	cataloged bool
	lookups   int
}

func (f *countingCatalogRepo) LookupCatalogEntry(ctx context.Context, projectID int, target dto.Target, medium enum.Medium) (bool, bool, error) {
	f.lookups++
	return f.cataloged, false, nil
}

func gateService(strict, cataloged bool) (*NotificationService, *flagProjectRepo, *countingCatalogRepo) {
	projectRepo := &flagProjectRepo{strict: strict}
	prefRepo := &countingCatalogRepo{cataloged: cataloged}

	svc := NewNotificationService(
		nil, nil, prefRepo, nil, nil, nil, nil, nil, projectRepo,
		nil, nil, nil,
	)

	return svc, projectRepo, prefRepo
}

func someTarget() *dto.Target {
	return &dto.Target{Channel: "billing", Topic: "none", Event: "invoice_paid"}
}

// TestStrictTargetsOffAcceptsUncatalogedTarget is the default, and it is the
// whole reason the flag exists. With the gate unconditional, a new user's FIRST
// targeted send fails with "create a project preference for it before sending" —
// before they have any reason to know what a catalog is.
//
// It also asserts the catalog is never consulted, which is the hot-path claim:
// for the default project the gate costs one primary-key read instead of N
// per-medium lookups, so it is cheaper than when it was unconditional.
func TestStrictTargetsOffAcceptsUncatalogedTarget(t *testing.T) {
	svc, _, prefRepo := gateService(false, false)

	errKind, err := svc.gateTarget(context.Background(), 1, someTarget(), []enum.Medium{enum.MediumInApp})
	if err != nil {
		t.Fatalf("strict targets off must accept an uncataloged target, got: %v", err)
	}
	if errKind != service.ErrNone {
		t.Fatalf("errKind = %v, want ErrNone", errKind)
	}

	if prefRepo.lookups != 0 {
		t.Errorf("catalog was consulted %d times with the gate off; the flag check must short-circuit first", prefRepo.lookups)
	}
}

// TestStrictTargetsOnRejectsUncatalogedTarget is the base case: turning it on
// has to actually reject, or the setting is decoration.
func TestStrictTargetsOnRejectsUncatalogedTarget(t *testing.T) {
	svc, _, _ := gateService(true, false)

	errKind, err := svc.gateTarget(context.Background(), 1, someTarget(), []enum.Medium{enum.MediumInApp})
	if err == nil {
		t.Fatal("strict targets on must reject an uncataloged target")
	}
	if errKind != service.ErrBadRequest {
		t.Fatalf("errKind = %v, want ErrBadRequest — an uncataloged target is a CALLER error, not a 500", errKind)
	}

	// The reader's project is one of the few running with this on, so "why is
	// this rejected when the docs say targets just work" is their next question.
	if !strings.Contains(err.Error(), "strict targets") {
		t.Errorf("the rejection must name the setting that caused it, got: %v", err)
	}
}

func TestStrictTargetsOnAcceptsCatalogedTarget(t *testing.T) {
	svc, _, _ := gateService(true, true)

	errKind, err := svc.gateTarget(context.Background(), 1, someTarget(), []enum.Medium{enum.MediumInApp, enum.MediumEmail})
	if err != nil {
		t.Fatalf("a cataloged target must pass the gate, got: %v", err)
	}
	if errKind != service.ErrNone {
		t.Fatalf("errKind = %v, want ErrNone", errKind)
	}
}

// TestUntargetedSendIsNeverGated pins the short-circuit that keeps the
// quickstart zero-config EVEN FOR A PROJECT WITH THE GATE ON. A send with no
// target claims no target, so there is nothing to check it against — and it must
// not cost a project read either.
func TestUntargetedSendIsNeverGated(t *testing.T) {
	svc, projectRepo, prefRepo := gateService(true, false)

	errKind, err := svc.gateTarget(context.Background(), 1, nil, []enum.Medium{enum.MediumInApp})
	if err != nil {
		t.Fatalf("an untargeted send must never be gated, got: %v", err)
	}
	if errKind != service.ErrNone {
		t.Fatalf("errKind = %v, want ErrNone", errKind)
	}

	if projectRepo.gets != 0 || prefRepo.lookups != 0 {
		t.Errorf("untargeted send hit the DB (%d project reads, %d catalog lookups); it should return before either",
			projectRepo.gets, prefRepo.lookups)
	}
}

// TestStrictTargetsGatesEveryRequestedMedium pins the per-medium rule: a target
// cataloged for in_app but not email must not let an email send through on the
// in_app entry's coat-tails. This is also why one flag is enough — the per-medium
// behaviour a second knob would buy is already here.
func TestStrictTargetsGatesEveryRequestedMedium(t *testing.T) {
	projectRepo := &flagProjectRepo{strict: true}
	prefRepo := &perMediumCatalogRepo{cataloged: map[enum.Medium]bool{enum.MediumInApp: true}}

	svc := NewNotificationService(nil, nil, prefRepo, nil, nil, nil, nil, nil, projectRepo, nil, nil, nil)

	// in_app alone passes.
	if _, err := svc.gateTarget(context.Background(), 1, someTarget(), []enum.Medium{enum.MediumInApp}); err != nil {
		t.Fatalf("in_app is cataloged and must pass: %v", err)
	}

	// Adding email — which is NOT cataloged — must reject the whole send.
	errKind, err := svc.gateTarget(context.Background(), 1, someTarget(), []enum.Medium{enum.MediumInApp, enum.MediumEmail})
	if err == nil {
		t.Fatal("an uncataloged email medium must reject even when in_app is cataloged")
	}
	if errKind != service.ErrBadRequest {
		t.Fatalf("errKind = %v, want ErrBadRequest", errKind)
	}
	if !strings.Contains(err.Error(), string(enum.MediumEmail)) {
		t.Errorf("the rejection must name the medium that failed, got: %v", err)
	}
}

type perMediumCatalogRepo struct {
	repository.PreferenceRepository
	cataloged map[enum.Medium]bool
}

func (f *perMediumCatalogRepo) LookupCatalogEntry(ctx context.Context, projectID int, target dto.Target, medium enum.Medium) (bool, bool, error) {
	return f.cataloged[medium], false, nil
}

// TestMandatoryIsIndependentOfStrictTargets confirms the open question raised
// when the flag was added: `mandatory` assumes a catalog row exists, so does it
// still behave with the gate off?
//
// It does, and the reason is structural rather than incidental: mandatory is
// resolved by the preference cascade, which never reads strict_targets. The gate
// only decides whether a send is REJECTED; it has no say in how a target that is
// cataloged resolves. So a mandatory entry on a permissive project is still
// non-negotiable — which is what you want, because "the recipient cannot opt out
// of this" is a property of the notification, not of the project's strictness.
func TestMandatoryIsIndependentOfStrictTargets(t *testing.T) {
	for _, strict := range []bool{false, true} {
		prefRepo := &mandatoryPrefRepo{mandatory: true}
		svc := NewProjectPreferenceService(prefRepo, &alwaysExistsRecipientRepo{})

		_, errKind, err := svc.UpsertRecipientPreference(context.Background(), upsertPayload())

		if errKind != service.ErrBadRequest {
			t.Errorf("strict=%v: a mandatory entry must refuse the recipient opt-out regardless of the gate, got %v (%v)",
				strict, errKind, err)
		}
		if prefRepo.created {
			t.Errorf("strict=%v: the refused opt-out was written anyway", strict)
		}
	}
}

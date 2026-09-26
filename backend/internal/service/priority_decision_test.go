package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/config"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/dto"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/model"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestPriorityDecisionVersionedIndependentReview(t *testing.T) {
	service := newPriorityTestService(t)
	ctx := context.Background()
	created, err := service.Create(ctx, priorityCreateInput("PD-TEST", "evidence-v1"), "operator", "req-create")
	if err != nil {
		t.Fatalf("create decision: %v", err)
	}
	if created.PreparedBy != "operator" || created.Version != 1 || len(created.Revisions) != 1 {
		t.Fatalf("unexpected created decision: %+v", created)
	}

	updated, err := service.Update(ctx, created.ID, priorityUpdateInput(created.Version, "evidence-v2"), "operator", model.RoleOperator, "req-update")
	if err != nil {
		t.Fatalf("update decision: %v", err)
	}
	if updated.Version != 2 || len(updated.Revisions) != 2 {
		t.Fatalf("expected two immutable revisions, got version=%d revisions=%d", updated.Version, len(updated.Revisions))
	}

	transition := dto.TransitionRequest{Status: "urgent", ExpectedVersion: updated.Version, Reason: "independent safety review"}
	if _, err := service.Transition(ctx, updated.ID, transition, "operator", model.RoleOperator, "req-operator-final"); !errors.Is(err, ErrReviewRole) {
		t.Fatalf("operator finalization should fail with review role error, got %v", err)
	}
	if _, err := service.Transition(ctx, updated.ID, transition, "operator", model.RoleReviewer, "req-self-final"); !errors.Is(err, ErrSeparationOfDuty) {
		t.Fatalf("preparer self-approval should fail, got %v", err)
	}

	finalized, err := service.Transition(ctx, updated.ID, transition, "reviewer", model.RoleReviewer, "req-final")
	if err != nil {
		t.Fatalf("independent review: %v", err)
	}
	if finalized.Status != "urgent" || finalized.Version != 3 || len(finalized.Revisions) != 3 {
		t.Fatalf("unexpected finalized decision: %+v", finalized)
	}
	wantEvidence := []string{"evidence-v1", "evidence-v2", "evidence-v2"}
	wantActors := []string{"operator", "operator", "reviewer"}
	wantRequests := []string{"req-create", "req-update", "req-final"}
	for index, revision := range finalized.Revisions {
		if revision.Version != uint(index+1) || revision.Evidence != wantEvidence[index] || revision.Actor != wantActors[index] || revision.RequestID != wantRequests[index] || revision.Snapshot == "" {
			t.Fatalf("revision %d lost audit evidence: %+v", index+1, revision)
		}
	}
	if _, err := service.Update(ctx, finalized.ID, priorityUpdateInput(finalized.Version, "late overwrite"), "operator", model.RoleOperator, "req-late"); !errors.Is(err, ErrDecisionLocked) {
		t.Fatalf("final decision must be immutable, got %v", err)
	}
}

func newPriorityTestService(t *testing.T) PriorityDecisionService {
	t.Helper()
	dsn := fmt.Sprintf("file:priority-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.PriorityDecision{}, &model.PriorityDecisionRevision{}, &model.DefectFinding{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	security := NewSecurityService(repository.NewSecurityRepository(db), config.Config{})
	return NewPriorityDecisionService(repository.NewPriorityDecisionRepository(db), repository.NewDefectFindingRepository(db), security)
}

func priorityCreateInput(code, evidence string) dto.CreatePriorityDecision {
	return dto.CreatePriorityDecision{
		Code: code, Name: "桥梁缺陷处置决定", Description: "versioning test", Facility: "K42 bridge",
		Owner: "infrastructure team", Category: "structural", RiskLevel: "critical", MetricValue: 87,
		MetricUnit: "score", EffectiveAt: time.Now().UTC(), Evidence: evidence, RelatedCode: "DF-TEST",
	}
}

func priorityUpdateInput(version uint, evidence string) dto.UpdatePriorityDecision {
	return dto.UpdatePriorityDecision{
		ExpectedVersion: version, Name: "桥梁缺陷处置决定", Description: "updated version", Facility: "K42 bridge",
		Owner: "infrastructure team", Category: "structural", RiskLevel: "critical", MetricValue: 92,
		MetricUnit: "score", EffectiveAt: time.Now().UTC(), Evidence: evidence, RelatedCode: "DF-TEST",
	}
}

func newReleaseTestEnv(t *testing.T) (PriorityDecisionService, repository.DefectFindingRepository, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:release-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.PriorityDecision{}, &model.PriorityDecisionRevision{}, &model.DefectFinding{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	defectRepo := repository.NewDefectFindingRepository(db)
	security := NewSecurityService(repository.NewSecurityRepository(db), config.Config{})
	service := NewPriorityDecisionService(repository.NewPriorityDecisionRepository(db), defectRepo, security)
	return service, defectRepo, db
}

func finalizeRestrictDecision(t *testing.T, ctx context.Context, service PriorityDecisionService) model.PriorityDecision {
	t.Helper()
	created, err := service.Create(ctx, priorityCreateInput("PD-REL", "evidence"), "operator", "req-create")
	if err != nil {
		t.Fatalf("create decision: %v", err)
	}
	finalized, err := service.Transition(ctx, created.ID,
		dto.TransitionRequest{Status: "restrict", ExpectedVersion: created.Version, Reason: "independent review"},
		"reviewer", model.RoleReviewer, "req-final")
	if err != nil {
		t.Fatalf("finalize restrict decision: %v", err)
	}
	return finalized
}

func seedDefect(t *testing.T, ctx context.Context, repo repository.DefectFindingRepository, code, status string) {
	t.Helper()
	defect := model.DefectFinding{
		BaseModel: model.BaseModel{Code: code, Name: "桥梁缺陷", Status: status, Version: 1},
		Facility:  "K42 bridge", Owner: "infrastructure team", RelatedCode: "REL-X",
	}
	if err := repo.Create(ctx, &defect); err != nil {
		t.Fatalf("create defect %s: %v", code, err)
	}
}

func TestPriorityDecisionReleaseGate(t *testing.T) {
	service, defectRepo, db := newReleaseTestEnv(t)
	ctx := context.Background()
	finalized := finalizeRestrictDecision(t, ctx, service)

	// No defects at all: clearance cannot be confirmed, release blocked.
	if _, err := service.Release(ctx, finalized.ID,
		dto.ReleasePriorityDecision{ExpectedVersion: finalized.Version, Reason: "all defects cleared"},
		"reviewer", model.RoleReviewer, "req-release"); !errors.Is(err, ErrNoBridgeDefects) {
		t.Fatalf("release without any defect evidence should fail, got %v", err)
	}

	// One verified defect: not release-ready, flag withdrawn and code reported.
	seedDefect(t, ctx, defectRepo, "DF-OPEN", "verified")
	view, err := service.Get(ctx, finalized.ID)
	if err != nil {
		t.Fatalf("get decision: %v", err)
	}
	if view.ReleaseReady {
		t.Fatal("decision must not be release-ready while a verified defect is open")
	}
	if !view.ActiveRequirement || len(view.PendingDefectCodes) != 1 || view.PendingDefectCodes[0] != "DF-OPEN" {
		t.Fatalf("expected pending DF-OPEN, got active=%v pending=%v", view.ActiveRequirement, view.PendingDefectCodes)
	}
	if _, err := service.Release(ctx, finalized.ID,
		dto.ReleasePriorityDecision{ExpectedVersion: finalized.Version, Reason: "all defects cleared"},
		"reviewer", model.RoleReviewer, "req-release"); !errors.Is(err, ErrDefectsPending) {
		t.Fatalf("release with pending defect should fail, got %v", err)
	}

	// The preparer can never release their own decision even once cleared.
	open := model.DefectFinding{}
	if err := db.WithContext(ctx).Where("code = ?", "DF-OPEN").First(&open).Error; err != nil {
		t.Fatalf("load defect: %v", err)
	}
	if err := defectRepo.Update(ctx, open.ID, open.Version,
		&model.DefectFinding{BaseModel: model.BaseModel{ID: open.ID, Code: open.Code, Name: open.Name, Status: "mitigated", Version: open.Version + 1},
			Facility: open.Facility, Owner: open.Owner, RelatedCode: open.RelatedCode}); err != nil {
		t.Fatalf("mitigate defect: %v", err)
	}
	view, err = service.Get(ctx, finalized.ID)
	if err != nil {
		t.Fatalf("refresh decision: %v", err)
	}
	if !view.ReleaseReady || len(view.PendingDefectCodes) != 0 {
		t.Fatalf("decision should be release-ready after mitigation, got ready=%v pending=%v", view.ReleaseReady, view.PendingDefectCodes)
	}
	if _, err := service.Release(ctx, finalized.ID,
		dto.ReleasePriorityDecision{ExpectedVersion: finalized.Version, Reason: "preparer self release"},
		"operator", model.RoleReviewer, "req-self"); !errors.Is(err, ErrSeparationOfDuty) {
		t.Fatalf("preparer release must be rejected, got %v", err)
	}

	// Independent reviewer releases it.
	released, err := service.Release(ctx, finalized.ID,
		dto.ReleasePriorityDecision{ExpectedVersion: finalized.Version, Reason: "bridge defects mitigated, lifting speed restriction"},
		"reviewer", model.RoleReviewer, "req-release")
	if err != nil {
		t.Fatalf("release decision: %v", err)
	}
	if released.Status != model.PriorityDecisionStatusReleased || released.Version != finalized.Version+1 {
		t.Fatalf("unexpected released decision: %+v", released)
	}
	last := released.Revisions[len(released.Revisions)-1]
	if last.Status != "released" || last.Actor != "reviewer" || last.RequestID != "req-release" {
		t.Fatalf("release revision not recorded: %+v", last)
	}
	view, _ = service.Get(ctx, finalized.ID)
	if view.ActiveRequirement || view.ReleaseReady {
		t.Fatal("released decision must no longer be an active requirement")
	}

	// Releasing again is rejected.
	if _, err := service.Release(ctx, finalized.ID,
		dto.ReleasePriorityDecision{ExpectedVersion: released.Version, Reason: "double release"},
		"reviewer", model.RoleReviewer, "req-double"); !errors.Is(err, ErrAlreadyReleased) {
		t.Fatalf("double release must fail, got %v", err)
	}
}

func TestReleaseReadinessWithdrawnWhenNewDefectConfirmed(t *testing.T) {
	service, defectRepo, _ := newReleaseTestEnv(t)
	ctx := context.Background()
	finalized := finalizeRestrictDecision(t, ctx, service)
	seedDefect(t, ctx, defectRepo, "DF-DONE", "closed")

	view, err := service.Get(ctx, finalized.ID)
	if err != nil {
		t.Fatalf("get decision: %v", err)
	}
	if !view.ReleaseReady {
		t.Fatal("decision with a closed defect should be release-ready")
	}

	// A freshly confirmed defect on the same bridge withdraws the flag.
	seedDefect(t, ctx, defectRepo, "DF-NEW", "verified")
	view, err = service.Get(ctx, finalized.ID)
	if err != nil {
		t.Fatalf("refresh decision: %v", err)
	}
	if view.ReleaseReady {
		t.Fatal("release-ready flag must be withdrawn once a new defect is confirmed")
	}
	if len(view.PendingDefectCodes) != 1 || view.PendingDefectCodes[0] != "DF-NEW" {
		t.Fatalf("only the new defect should remain pending, got %v", view.PendingDefectCodes)
	}
}

func TestObserveDecisionIsNotReleasable(t *testing.T) {
	service, defectRepo, _ := newReleaseTestEnv(t)
	ctx := context.Background()
	created, err := service.Create(ctx, priorityCreateInput("PD-OBS", "evidence"), "operator", "req-create")
	if err != nil {
		t.Fatalf("create decision: %v", err)
	}
	observed, err := service.Transition(ctx, created.ID,
		dto.TransitionRequest{Status: "observe", ExpectedVersion: created.Version, Reason: "review"},
		"reviewer", model.RoleReviewer, "req-final")
	if err != nil {
		t.Fatalf("finalize observe: %v", err)
	}
	seedDefect(t, ctx, defectRepo, "DF-C", "closed")
	view, err := service.Get(ctx, observed.ID)
	if err != nil {
		t.Fatalf("get decision: %v", err)
	}
	if view.ReleaseReady || view.ActiveRequirement {
		t.Fatal("an observe decision imposes no restriction and is not release-ready")
	}
	if _, err := service.Release(ctx, observed.ID,
		dto.ReleasePriorityDecision{ExpectedVersion: observed.Version, Reason: "try release observe"},
		"reviewer", model.RoleReviewer, "req-release"); !errors.Is(err, ErrReleaseNotActive) {
		t.Fatalf("releasing an observe decision must fail, got %v", err)
	}
}

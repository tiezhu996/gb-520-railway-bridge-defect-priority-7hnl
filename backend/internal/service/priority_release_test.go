package service

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/constants"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/dto"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/model"
	"gorm.io/gorm"
)

// seedDefect inserts a defect on the given bridge (facility) with the given
// lifecycle state.
func seedDefect(t *testing.T, db *gorm.DB, code, facility, status string) {
	t.Helper()
	defect := model.DefectFinding{
		BaseModel: model.BaseModel{
			Code: code, Name: "桥梁缺陷 " + code, Status: status, Version: 1,
		},
		Facility:    facility,
		Owner:       "现场处置组",
		Category:    "结构",
		RiskLevel:   "high",
		EffectiveAt: time.Now().UTC(),
		Evidence:    "现场量测记录",
		RelatedCode: "IR-REL",
	}
	if err := db.Create(&defect).Error; err != nil {
		t.Fatalf("seed defect %s: %v", code, err)
	}
}

func finalizeRestrict(t *testing.T, service PriorityDecisionService, input dto.CreatePriorityDecision) model.PriorityDecision {
	t.Helper()
	ctx := context.Background()
	created, err := service.Create(ctx, input, "operator", "req-create")
	if err != nil {
		t.Fatalf("create decision: %v", err)
	}
	finalized, err := service.Transition(ctx, created.ID, dto.TransitionRequest{
		Status: "restrict", ExpectedVersion: created.Version, Reason: "独立复核确认限速",
	}, "reviewer", model.RoleReviewer, "req-final")
	if err != nil {
		t.Fatalf("finalize restrict: %v", err)
	}
	return finalized
}

func TestReleaseMarkerTracksDefectsOnSameBridge(t *testing.T) {
	service, db := newPriorityTestService(t)
	ctx := context.Background()
	facility := "K55 桥梁作业区"

	input := priorityCreateInput("PD-REL-1", "裂缝证据")
	input.Facility = facility
	finalized := finalizeRestrict(t, service, input)

	// One outstanding defect keeps the decision in force.
	seedDefect(t, db, "DF-REL-1", facility, "verified")
	got, err := service.Get(ctx, finalized.ID)
	if err != nil {
		t.Fatalf("get decision: %v", err)
	}
	if got.ReleaseEligible {
		t.Fatal("decision must not be releasable while a defect is verified")
	}
	if len(got.OutstandingDefectCodes) != 1 || got.OutstandingDefectCodes[0] != "DF-REL-1" {
		t.Fatalf("expected outstanding [DF-REL-1], got %#v", got.OutstandingDefectCodes)
	}

	// A defect on another bridge must not appear in the blocking list.
	seedDefect(t, db, "DF-OTHER", "K99 另一座桥", "new")
	got, _ = service.Get(ctx, finalized.ID)
	if len(got.OutstandingDefectCodes) != 1 {
		t.Fatalf("other bridge defect must be excluded, got %#v", got.OutstandingDefectCodes)
	}

	// Mitigating the defect flips the marker; closing keeps it releasable.
	db.Model(&model.DefectFinding{}).Where("code = ?", "DF-REL-1").Update("status", "mitigated")
	got, _ = service.Get(ctx, finalized.ID)
	if !got.ReleaseEligible || len(got.OutstandingDefectCodes) != 0 {
		t.Fatalf("expected releasable with no outstanding, got eligible=%v codes=%#v", got.ReleaseEligible, got.OutstandingDefectCodes)
	}
}

func TestReleaseRequiresAllDefectsResolved(t *testing.T) {
	service, db := newPriorityTestService(t)
	ctx := context.Background()
	facility := "K66 桥梁作业区"

	input := priorityCreateInput("PD-REL-2", "限速证据")
	input.Facility = facility
	finalized := finalizeRestrict(t, service, input)

	seedDefect(t, db, "DF-A", facility, "new")
	seedDefect(t, db, "DF-B", facility, "monitoring")

	// Preparer cannot release their own decision even with the right role.
	if _, err := service.Release(ctx, finalized.ID, dto.ReleasePriorityDecision{ExpectedVersion: finalized.Version, Reason: "缺陷已全部处理"}, "operator", model.RoleReviewer, "req-self"); !errors.Is(err, ErrSeparationOfDuty) {
		t.Fatalf("preparer release should fail separation of duty, got %v", err)
	}
	// Operator role cannot release.
	if _, err := service.Release(ctx, finalized.ID, dto.ReleasePriorityDecision{ExpectedVersion: finalized.Version, Reason: "缺陷已全部处理"}, "operator2", model.RoleOperator, "req-op"); !errors.Is(err, ErrReleaseRole) {
		t.Fatalf("operator release should fail with release role error, got %v", err)
	}
	// Independent reviewer is blocked while defects remain open; codes are listed.
	_, err := service.Release(ctx, finalized.ID, dto.ReleasePriorityDecision{ExpectedVersion: finalized.Version, Reason: "缺陷已全部处理"}, "reviewer", model.RoleReviewer, "req-blocked")
	if !errors.Is(err, ErrReleaseBlocked) {
		t.Fatalf("release with outstanding defects should fail, got %v", err)
	}

	// Resolve every defect on the bridge.
	db.Model(&model.DefectFinding{}).Where("code IN ?", []string{"DF-A", "DF-B"}).Update("status", "closed")
	released, err := service.Release(ctx, finalized.ID, dto.ReleasePriorityDecision{ExpectedVersion: finalized.Version, Reason: "同桥缺陷均已关闭，解除限速"}, "reviewer", model.RoleReviewer, "req-release")
	if err != nil {
		t.Fatalf("release after resolution: %v", err)
	}
	if released.Status != constants.PriorityStatusReleased || released.Version != finalized.Version+1 {
		t.Fatalf("unexpected released decision: %+v", released)
	}
	if released.ReleaseEligible {
		t.Fatal("a released decision must no longer be marked releasable")
	}
	if len(released.Revisions) != 3 {
		t.Fatalf("expected draft/final/release revisions, got %d", len(released.Revisions))
	}
	last := released.Revisions[len(released.Revisions)-1]
	if last.Status != constants.PriorityStatusReleased || last.Actor != "reviewer" || last.RequestID != "req-release" {
		t.Fatalf("release revision not preserved: %+v", last)
	}

	// A released decision cannot be released again.
	if _, err := service.Release(ctx, finalized.ID, dto.ReleasePriorityDecision{ExpectedVersion: released.Version, Reason: "再次解除"}, "admin", model.RoleAdmin, "req-again"); !errors.Is(err, ErrReleaseNotAllowed) {
		t.Fatalf("re-release should be rejected, got %v", err)
	}
}

func TestNewConfirmedDefectWithdrawsReleaseMarker(t *testing.T) {
	service, db := newPriorityTestService(t)
	ctx := context.Background()
	facility := "K77 桥梁作业区"

	input := priorityCreateInput("PD-REL-3", "立即处置证据")
	input.Facility = facility
	input.RiskLevel = "critical"
	created, err := service.Create(ctx, input, "operator", "req-create")
	if err != nil {
		t.Fatalf("create decision: %v", err)
	}
	finalized, err := service.Transition(ctx, created.ID, dto.TransitionRequest{
		Status: "urgent", ExpectedVersion: created.Version, Reason: "独立复核确认立即处置",
	}, "reviewer", model.RoleReviewer, "req-final")
	if err != nil {
		t.Fatalf("finalize urgent: %v", err)
	}

	// The only defect is already mitigated, so the workbench marks the decision
	// releasable.
	seedDefect(t, db, "DF-DONE", facility, "mitigated")
	got, _ := service.Get(ctx, finalized.ID)
	if !got.ReleaseEligible {
		t.Fatal("urgent decision should be releasable when the only defect is mitigated")
	}

	// Before anyone releases it, a newly confirmed defect on the same bridge
	// withdraws the marker and shows up in the outstanding list.
	seedDefect(t, db, "DF-NEW", facility, "verified")
	got, _ = service.Get(ctx, finalized.ID)
	if got.ReleaseEligible {
		t.Fatal("newly confirmed defect must withdraw the release marker")
	}
	if len(got.OutstandingDefectCodes) != 1 || got.OutstandingDefectCodes[0] != "DF-NEW" {
		t.Fatalf("expected only DF-NEW outstanding, got %#v", got.OutstandingDefectCodes)
	}
	// The reviewer must be refused while that defect is open.
	if _, err := service.Release(ctx, finalized.ID, dto.ReleasePriorityDecision{ExpectedVersion: finalized.Version, Reason: "尝试解除立即处置"}, "reviewer", model.RoleReviewer, "req-early"); !errors.Is(err, ErrReleaseBlocked) {
		t.Fatalf("release must be blocked by the new defect, got %v", err)
	}

	// Resolve the new defect; the marker returns and release succeeds.
	db.Model(&model.DefectFinding{}).Where("code = ?", "DF-NEW").Update("status", "closed")
	released, err := service.Release(ctx, finalized.ID, dto.ReleasePriorityDecision{ExpectedVersion: finalized.Version, Reason: "无遗留缺陷，解除立即处置"}, "reviewer", model.RoleReviewer, "req-release")
	if err != nil {
		t.Fatalf("release urgent: %v", err)
	}
	if released.Status != constants.PriorityStatusReleased {
		t.Fatalf("expected released, got %s", released.Status)
	}

	// A confirmed defect appearing after release does not resurrect the marker
	// (the decision is already a non-current released requirement).
	seedDefect(t, db, "DF-LATER", facility, "new")
	got, _ = service.Get(ctx, finalized.ID)
	if got.ReleaseEligible || got.Status != constants.PriorityStatusReleased {
		t.Fatal("released decision must stay released and not become releasable again")
	}
}

func TestObserveDecisionIsNeverReleasable(t *testing.T) {
	service, _ := newPriorityTestService(t)
	ctx := context.Background()

	input := priorityCreateInput("PD-REL-4", "观察证据")
	created, err := service.Create(ctx, input, "operator", "req-create")
	if err != nil {
		t.Fatalf("create decision: %v", err)
	}
	observed, err := service.Transition(ctx, created.ID, dto.TransitionRequest{
		Status: "observe", ExpectedVersion: created.Version, Reason: "观察即可",
	}, "reviewer", model.RoleReviewer, "req-final")
	if err != nil {
		t.Fatalf("finalize observe: %v", err)
	}
	if _, err := service.Release(ctx, observed.ID, dto.ReleasePriorityDecision{ExpectedVersion: observed.Version, Reason: "尝试解除观察"}, "reviewer", model.RoleReviewer, "req-release"); !errors.Is(err, ErrReleaseNotAllowed) {
		t.Fatalf("observe decision cannot be released, got %v", err)
	}
	got, _ := service.Get(ctx, observed.ID)
	if got.ReleaseEligible || len(got.OutstandingDefectCodes) != 0 {
		t.Fatalf("observe must never carry a release marker, got %+v", got)
	}
}

func TestReleaseListEnrichmentCodesAreSorted(t *testing.T) {
	service, db := newPriorityTestService(t)
	ctx := context.Background()
	facility := "K88 桥梁作业区"

	input := priorityCreateInput("PD-REL-5", "限速证据")
	input.Facility = facility
	finalizeRestrict(t, service, input)
	seedDefect(t, db, "DF-Z", facility, "new")
	seedDefect(t, db, "DF-A", facility, "verified")

	page, err := service.List(ctx, dto.PageQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var codes []string
	for _, item := range page.Items {
		if item.Status == "restrict" {
			codes = append(codes, item.OutstandingDefectCodes...)
		}
	}
	if len(codes) != 2 {
		t.Fatalf("expected two outstanding codes, got %#v", codes)
	}
	if !sort.StringsAreSorted(codes) {
		t.Fatalf("expected codes sorted by repository, got %#v", codes)
	}
}

package constants

// Shared status values are mirrored in frontend/src/types/status.ts. Keeping
// the lists explicit makes state-machine drift visible during code review.

type DefectState string

const (
	DefectStateNew        DefectState = "new"
	DefectStateVerified   DefectState = "verified"
	DefectStateMonitoring DefectState = "monitoring"
	DefectStateMitigated  DefectState = "mitigated"
	DefectStateClosed     DefectState = "closed"
)

var AllDefectState = []string{"new", "verified", "monitoring", "mitigated", "closed"}

type PriorityLevel string

const (
	PriorityLevelObserve  PriorityLevel = "observe"
	PriorityLevelRestrict PriorityLevel = "restrict"
	PriorityLevelUrgent   PriorityLevel = "urgent"
)

var AllPriorityLevel = []string{"observe", "restrict", "urgent"}

// PriorityStatusReleased is the terminal state reached after an independent
// reviewer closes out a finalized restrict/urgent decision because every
// defect on the same bridge is mitigated or closed. A released decision is no
// longer a current operational requirement.
const PriorityStatusReleased = "released"

// ResolvedDefectStates are the defect states that no longer keep a finalized
// speed restriction or urgent handling decision in force: a mitigated defect
// is controlled and a closed defect is fully dealt with.
var ResolvedDefectStates = map[string]bool{
	string(DefectStateMitigated): true,
	string(DefectStateClosed):    true,
}

// IsResolvedDefectState reports whether a defect counts as handled for the
// purpose of releasing a finalized priority decision.
func IsResolvedDefectState(status string) bool {
	return ResolvedDefectStates[status]
}

// ReleasablePriorityStatuses are the finalized decisions ("一条定稿过的限速或者
// 立即处置") that the workbench tracks for automatic close-out: restrict (限速)
// and urgent (立即处置). observe is an observation decision and is never
// released.
var ReleasablePriorityStatuses = map[string]bool{
	string(PriorityLevelRestrict): true,
	string(PriorityLevelUrgent):   true,
}

// IsReleasablePriorityStatus reports whether a finalized priority decision may
// become releasable and later move to released.
func IsReleasablePriorityStatus(status string) bool {
	return ReleasablePriorityStatuses[status]
}

var BridgeAssetTransitions = map[string]map[string]bool{
	"active":     {"restricted": true, "closed": true},
	"restricted": {"closed": true, "retired": true, "active": true},
	"closed":     {"retired": true, "restricted": true},
	"retired":    {"closed": true},
}

var InspectionRoundTransitions = map[string]map[string]bool{
	"planned":   {"running": true, "review": true},
	"running":   {"review": true, "completed": true, "planned": true},
	"review":    {"completed": true, "running": true},
	"completed": {"review": true},
}

var DefectFindingTransitions = map[string]map[string]bool{
	"new":        {"verified": true, "monitoring": true},
	"verified":   {"monitoring": true, "mitigated": true, "new": true},
	"monitoring": {"mitigated": true, "closed": true, "verified": true},
	"mitigated":  {"closed": true, "monitoring": true},
	"closed":     {"mitigated": true},
}

var PriorityDecisionTransitions = map[string]map[string]bool{
	"draft":    {"observe": true, "restrict": true, "urgent": true},
	"observe":  {},
	"restrict": {},
	"urgent":   {},
}

func CanTransition(graph map[string]map[string]bool, from, to string) bool {
	targets, exists := graph[from]
	return exists && targets[to]
}

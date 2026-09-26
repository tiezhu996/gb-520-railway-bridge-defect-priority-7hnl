package constants

import "testing"

func TestBridgeAssetTransitionGraph(t *testing.T) {
	if !CanTransition(BridgeAssetTransitions, "active", "restricted") {
		t.Fatalf("expected active -> restricted transition to be allowed")
	}
	if CanTransition(BridgeAssetTransitions, "active", "unknown") {
		t.Fatal("unknown status must never be accepted")
	}
}

func TestReleaseIsNotReachableViaGenericTransition(t *testing.T) {
	for _, from := range AllPriorityLevel {
		if CanTransition(PriorityDecisionTransitions, from, PriorityDecisionStatusReleased) {
			t.Fatalf("generic transition graph must not allow %s -> released; release requires the gated endpoint", from)
		}
	}
}

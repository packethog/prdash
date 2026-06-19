package pr

import (
	"reflect"
	"testing"
)

func TestMergeBlockers(t *testing.T) {
	mergeable := PR{ReviewDecision: "APPROVED", RollupState: "SUCCESS", Mergeable: "MERGEABLE"}
	if b := MergeBlockers(mergeable); len(b) != 0 {
		t.Errorf("expected no blockers, got %v", b)
	}
	if !CanMerge(mergeable) {
		t.Error("CanMerge should be true for approved+green+clean")
	}

	pending := PR{ReviewDecision: "", RollupState: "PENDING", Mergeable: "MERGEABLE"}
	if got, want := MergeBlockers(pending), []string{"review not approved", "CI not passing"}; !reflect.DeepEqual(got, want) {
		t.Errorf("MergeBlockers(pending) = %v, want %v", got, want)
	}
	if CanMerge(pending) {
		t.Error("CanMerge should be false for pending")
	}

	draft := PR{IsDraft: true, ReviewDecision: "", RollupState: ""}
	if got, want := MergeBlockers(draft), []string{"PR is a draft", "review not approved", "CI not passing", "mergeability unknown"}; !reflect.DeepEqual(got, want) {
		t.Errorf("MergeBlockers(draft) = %v, want %v", got, want)
	}

	conflicting := PR{ReviewDecision: "APPROVED", RollupState: "SUCCESS", Mergeable: "CONFLICTING"}
	if got, want := MergeBlockers(conflicting), []string{"merge conflicts present"}; !reflect.DeepEqual(got, want) {
		t.Errorf("MergeBlockers(conflicting) = %v, want %v", got, want)
	}

	unknown := PR{ReviewDecision: "APPROVED", RollupState: "SUCCESS", Mergeable: "UNKNOWN"}
	if got, want := MergeBlockers(unknown), []string{"mergeability unknown"}; !reflect.DeepEqual(got, want) {
		t.Errorf("MergeBlockers(unknown) = %v, want %v", got, want)
	}
	if CanMerge(unknown) {
		t.Error("CanMerge should be false for UNKNOWN mergeability")
	}
}

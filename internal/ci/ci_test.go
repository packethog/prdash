package ci

import "testing"

func TestStatus(t *testing.T) {
	cases := []struct {
		name       string
		status     string
		conclusion string
		want       RunStatus
	}{
		{"success", "completed", "success", RunSuccess},
		{"failure", "completed", "failure", RunFailure},
		{"timed_out", "completed", "timed_out", RunFailure},
		{"startup_failure", "completed", "startup_failure", RunFailure},
		{"cancelled", "completed", "cancelled", RunCancelled},
		{"in_progress", "in_progress", "", RunInProgress},
		{"queued", "queued", "", RunQueued},
		{"requested", "requested", "", RunQueued},
		{"waiting", "waiting", "", RunQueued},
		{"weird", "completed", "neutral", RunOther},
		{"unknown status", "banana", "", RunOther},
	}
	for _, c := range cases {
		got := Status(Run{Status: c.status, Conclusion: c.conclusion})
		if got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestIsFailed(t *testing.T) {
	if !IsFailed(Run{Status: "completed", Conclusion: "failure"}) {
		t.Error("failure run should be failed")
	}
	if IsFailed(Run{Status: "completed", Conclusion: "success"}) {
		t.Error("success run should not be failed")
	}
}

func TestSymbolLabel(t *testing.T) {
	if RunSuccess.Symbol() != "✓" || RunFailure.Symbol() != "✗" || RunCancelled.Symbol() != "⊘" {
		t.Error("symbols wrong")
	}
	if RunQueued.Symbol() != "◐" || RunInProgress.Symbol() != "◐" || RunOther.Symbol() != "–" {
		t.Error("pending/none symbols wrong")
	}
	if RunSuccess.Label() != "passed" || RunFailure.Label() != "failed" {
		t.Error("labels wrong")
	}
}

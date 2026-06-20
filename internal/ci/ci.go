// Package ci holds prdash's pure domain types for GitHub Actions workflow runs:
// the run model, status mapping, and badge glyphs/labels. No network or UI deps.
package ci

import "time"

// RunStatus is the displayed status of a workflow run.
type RunStatus int

const (
	RunOther RunStatus = iota
	RunQueued
	RunInProgress
	RunSuccess
	RunFailure
	RunCancelled
)

// Run is one workflow run as prdash needs it. Raw GitHub enum strings are kept
// verbatim so Status mapping can be tested independently of decoding.
type Run struct {
	Repo         string // owner/name
	WorkflowKey  string // workflow file name
	WorkflowName string // display label
	Branch       string
	Status       string // "queued", "in_progress", "completed", ...
	Conclusion   string // "success", "failure", "cancelled", ...
	URL          string
	RunID        int64 // databaseId, for `gh run ...`
	RunNumber    int   // GitHub run number
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// WorkflowRuns is one configured workflow plus its fetched last-N runs (newest
// first). Err is set when the fetch for this workflow failed (its row degrades).
type WorkflowRuns struct {
	Name   string
	Branch string
	Repo   string
	Key    string
	Runs   []Run
	Err    error
}

// Status maps GitHub's status/conclusion strings to a RunStatus.
func Status(r Run) RunStatus {
	if r.Status != "completed" {
		switch r.Status {
		case "in_progress":
			return RunInProgress
		case "queued", "requested", "waiting", "pending":
			return RunQueued
		default:
			return RunOther
		}
	}
	switch r.Conclusion {
	case "success":
		return RunSuccess
	case "failure", "timed_out", "startup_failure":
		return RunFailure
	case "cancelled":
		return RunCancelled
	default:
		return RunOther
	}
}

// IsFailed reports whether the run is debug/rerun eligible.
func IsFailed(r Run) bool { return Status(r) == RunFailure }

// Symbol is the single-cell status glyph.
func (s RunStatus) Symbol() string {
	switch s {
	case RunSuccess:
		return "✓"
	case RunFailure:
		return "✗"
	case RunCancelled:
		return "⊘"
	case RunQueued, RunInProgress:
		return "◐"
	default:
		return "–"
	}
}

// Label is the human-readable status word.
func (s RunStatus) Label() string {
	switch s {
	case RunSuccess:
		return "passed"
	case RunFailure:
		return "failed"
	case RunCancelled:
		return "cancelled"
	case RunQueued:
		return "queued"
	case RunInProgress:
		return "running"
	default:
		return "—"
	}
}

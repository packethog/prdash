package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/packethog/prdash/internal/ci"
)

// A long analysis line wraps to the terminal width instead of being truncated:
// every rendered line fits the width AND the trailing content is preserved.
func TestSummaryWrapsToWidth(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10, WithCI(testCIConfig(t)))
	m.width, m.height = 40, 40
	m.section = secCI
	m.workflows = []ci.WorkflowRuns{{Name: "QA", Key: "w.yml", Repo: "a/b", Runs: []ci.Run{
		{RunID: 9, RunNumber: 9, Status: "completed", Conclusion: "failure"},
	}}}
	m.expanded["a/b w.yml"] = true
	m.cursor = 1
	m.modal = modalDetails
	m.detailRun = m.workflows[0].Runs[0]
	m.summary = "This is a very long analysis line that definitely exceeds forty columns and must wrap across several rows instead of running off the edge."
	out := m.View()
	for _, ln := range strings.Split(out, "\n") {
		if ansi.StringWidth(ln) > m.width {
			t.Errorf("line exceeds width %d (w=%d): %q", m.width, ansi.StringWidth(ln), ln)
		}
	}
	if !strings.Contains(out, "edge.") { // trailing content survived (not truncated)
		t.Errorf("summary tail truncated rather than wrapped:\n%s", out)
	}
}

// Expanded run rows align under the columns: #number, branch, status glyph, and
// "<updated> ago (<runtime>)" — no status word, and branch lives on the run row.
func TestExpandedRunRowLayout(t *testing.T) {
	created := time.Date(2026, 6, 20, 18, 0, 0, 0, time.UTC)
	updated := created.Add(12 * time.Minute)
	m := New(stubRunner{}, time.Second, 10, WithCI(testCIConfig(t)))
	m.width, m.height = 120, 40
	m.now = func() time.Time { return updated.Add(3 * time.Minute) }
	m.section = secCI
	m.workflows = []ci.WorkflowRuns{{Name: "QA", Key: "w.yml", Repo: "a/b", Runs: []ci.Run{
		{RunID: 1, RunNumber: 4820, Branch: "main", Status: "completed", Conclusion: "failure", CreatedAt: created, UpdatedAt: updated},
	}}}
	m.expanded["a/b w.yml"] = true
	out := m.View()
	if !strings.Contains(out, "#4820") {
		t.Error("run number missing from run row")
	}
	if !strings.Contains(out, "main") {
		t.Error("branch should show on the run row")
	}
	if strings.Contains(out, "failed") {
		t.Errorf("status word should not be rendered (glyph only):\n%s", out)
	}
	if !strings.Contains(out, "3m ago") || !strings.Contains(out, "(12m)") {
		t.Errorf("updated time and runtime missing:\n%s", out)
	}
}

// shift+tab rotates sections in reverse (Authored → CI → Awaiting → Authored).
func TestShiftTabRotatesSectionsBackwards(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10, WithCI(testCIConfig(t)))
	m.workflows = []ci.WorkflowRuns{{Name: "QA", Key: "w.yml", Repo: "a/b"}}
	m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.section != secCI {
		t.Fatalf("after 1 shift+tab: %v, want secCI", m.section)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.section != secReviewing {
		t.Fatalf("after 2 shift+tab: %v, want secReviewing", m.section)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.section != secAuthored {
		t.Fatalf("after 3 shift+tab: %v, want secAuthored", m.section)
	}
}

// The LAST column scales to the run count: all status glyphs render and the
// UPDATED time still follows (no truncation) even past the old 7-glyph cap.
func TestSparklineScalesToRunCount(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10, WithCI(testCIConfig(t)))
	m.width, m.height = 200, 40
	updated := time.Date(2026, 6, 20, 18, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return updated.Add(3 * time.Minute) }
	runs := make([]ci.Run, 10) // more than the old ciSparkMax of 7
	for i := range runs {
		runs[i] = ci.Run{RunID: int64(i + 1), RunNumber: i + 1, Status: "completed", Conclusion: "success", UpdatedAt: updated}
	}
	m.section = secCI
	m.workflows = []ci.WorkflowRuns{{Name: "QA", Key: "w.yml", Repo: "a/b", Runs: runs}}
	out := m.View()
	// the collapsed row line holds all 10 glyphs followed by the updated time
	var row string
	for _, ln := range strings.Split(out, "\n") {
		if strings.Contains(ln, "QA") && strings.Contains(ln, "✓") {
			row = ln
		}
	}
	if got := strings.Count(row, "✓"); got != 10 {
		t.Errorf("collapsed sparkline shows %d glyphs, want 10:\n%s", got, row)
	}
	if !strings.Contains(row, "3m ago") {
		t.Errorf("updated time cut off by the sparkline:\n%s", row)
	}
}

// Opening a run's details renders the panel inline beneath the run without wiping
// the rest of the window (the section header and run row stay visible).
func TestDetailsRenderInline(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10, WithCI(testCIConfig(t)))
	m.width, m.height = 120, 40
	m.section = secCI
	m.workflows = []ci.WorkflowRuns{{Name: "QA", Key: "w.yml", Repo: "a/b", Runs: []ci.Run{
		{RunID: 9, RunNumber: 9, Branch: "main", Status: "completed", Conclusion: "failure"},
	}}}
	m.expanded["a/b w.yml"] = true
	m.cursor = 1 // the run row
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.modal != modalDetails {
		t.Fatalf("expected modalDetails, got %v", m.modal)
	}
	out := m.View()
	if !strings.Contains(out, "CI Workflows") {
		t.Errorf("window was wiped — section header missing:\n%s", out)
	}
	if !strings.Contains(out, "#9") {
		t.Error("run row should remain visible above its details")
	}
	if !strings.Contains(out, "loading details") {
		t.Errorf("inline details panel not rendered under the run:\n%s", out)
	}
}

// The debug hint follows the PR-section convention: visible whenever the CI
// section is focused under cmux with a configured provider, regardless of the
// selected run's state.
func TestDebugHintVisibleInCmux(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "1")
	m := New(stubRunner{}, time.Second, 10, WithCI(testCIConfig(t)))
	m.width, m.height = 120, 40
	m.section = secCI
	m.workflows = []ci.WorkflowRuns{{Name: "QA", Key: "w.yml", Repo: "a/b", Runs: []ci.Run{
		{RunID: 1, RunNumber: 1, Status: "completed", Conclusion: "success"}, // even a passing run
	}}}
	out := m.View()
	if !strings.Contains(out, "d debug") {
		t.Errorf("debug hint should show in CI section under cmux:\n%s", out)
	}
}

// Outside cmux the debug option is gated off, like the PR review launcher.
func TestDebugHintHiddenOutsideCmux(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "")
	m := New(stubRunner{}, time.Second, 10, WithCI(testCIConfig(t)))
	m.width, m.height = 120, 40
	m.section = secCI
	m.workflows = []ci.WorkflowRuns{{Name: "QA", Key: "w.yml", Repo: "a/b", Runs: []ci.Run{
		{RunID: 2, RunNumber: 2, Status: "completed", Conclusion: "failure"},
	}}}
	out := m.View()
	if strings.Contains(out, "d debug") {
		t.Errorf("debug hint must be cmux-gated:\n%s", out)
	}
}

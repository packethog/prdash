package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/packethog/prdash/internal/ci"
)

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

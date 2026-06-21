package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/packethog/prdash/internal/ci"
)

// TestCIFetchGenDropsStale verifies that a ciFetchedMsg whose gen field is
// older than the model's current ciGen is silently discarded, and that a
// matching gen is applied.
func TestCIFetchGenDropsStale(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10, WithCI(testCIConfig(t)))
	m.ciGen = 3

	stale := []ci.WorkflowRuns{{Name: "old", Key: "w.yml", Repo: "a/b"}}
	fresh := []ci.WorkflowRuns{{Name: "new", Key: "w.yml", Repo: "a/b"}}

	// stale gen: workflows must not be updated
	m.Update(ciFetchedMsg{gen: 1, workflows: stale})
	if len(m.workflows) != 0 {
		t.Fatalf("stale gen: workflows should be untouched, got %v", m.workflows)
	}

	// current gen: workflows must be applied
	m.Update(ciFetchedMsg{gen: 3, workflows: fresh})
	if len(m.workflows) != 1 || m.workflows[0].Name != "new" {
		t.Fatalf("current gen: workflows not applied: %v", m.workflows)
	}
}

func TestEnterTogglesExpand(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10, WithCI(testCIConfig(t)))
	m.section = secCI
	m.workflows = []ci.WorkflowRuns{{Name: "QA", Key: "w.yml", Repo: "a/b", Runs: []ci.Run{{RunID: 1, Status: "completed", Conclusion: "failure"}}}}
	m.cursor = 0 // on the header
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.expanded["a/b w.yml"] {
		t.Fatal("workflow should be expanded")
	}
	// now there are 2 items (header + 1 run)
	if len(m.ciItems()) != 2 {
		t.Fatalf("want 2 items, got %d", len(m.ciItems()))
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.expanded["a/b w.yml"] {
		t.Fatal("workflow should be collapsed again")
	}
}

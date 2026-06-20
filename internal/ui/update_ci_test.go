package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/packethog/prdash/internal/ci"
)

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

package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/packethog/prdash/internal/ci"
	"github.com/packethog/prdash/internal/gh"
)

func TestEnterOnRunOpensDetails(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10, WithCI(testCIConfig(t)))
	m.section = secCI
	m.workflows = []ci.WorkflowRuns{{Name: "QA", Key: "w.yml", Repo: "a/b",
		Runs: []ci.Run{{RunID: 9, RunNumber: 9, Status: "completed", Conclusion: "failure"}}}}
	m.expanded["a/b w.yml"] = true
	m.cursor = 1 // the run row
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.modal != modalDetails {
		t.Fatalf("modal = %v, want details", m.modal)
	}
	if m.detailRun.RunID != 9 {
		t.Fatalf("detailRun not captured: %+v", m.detailRun)
	}
}

func TestSummaryMsgPopulates(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10, WithCI(testCIConfig(t)))
	m.modal = modalDetails
	m.Update(summaryMsg{data: []byte("analysis here")})
	if m.summary != "analysis here" {
		t.Fatalf("summary = %q", m.summary)
	}
}

func TestSummaryMsgNoArtifact(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10, WithCI(testCIConfig(t)))
	m.modal = modalDetails
	m.Update(summaryMsg{err: gh.ErrNoArtifact})
	if m.summaryErr == nil {
		t.Fatal("summaryErr should be set")
	}
}

func TestDetailsEscCloses(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10, WithCI(testCIConfig(t)))
	m.modal = modalDetails
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.modal != modalNone {
		t.Fatal("esc should close details")
	}
}

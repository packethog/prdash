package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/packethog/prdash/internal/ci"
)

func failingCIModel(t *testing.T) *Model {
	m := New(stubRunner{}, time.Second, 10, WithCI(testCIConfig(t)))
	m.section = secCI
	m.workflows = []ci.WorkflowRuns{{Name: "QA", Key: "w.yml", Repo: "a/b",
		Runs: []ci.Run{
			{RunID: 1, RunNumber: 1, Status: "completed", Conclusion: "success"},
			{RunID: 2, RunNumber: 2, Status: "completed", Conclusion: "failure"},
		}}}
	m.expanded["a/b w.yml"] = true
	return m
}

func TestRerunOnFailedOpensModal(t *testing.T) {
	m := failingCIModel(t)
	m.cursor = 2 // header=0, run1=1, run2=2 (failed)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	if m.modal != modalRerun || m.rerunRun.RunID != 2 {
		t.Fatalf("rerun modal not armed: modal=%v run=%+v", m.modal, m.rerunRun)
	}
}

func TestRerunIgnoredOnSuccessRun(t *testing.T) {
	m := failingCIModel(t)
	m.cursor = 1 // the success run
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	if m.modal != modalNone {
		t.Fatal("R on a passing run should do nothing")
	}
}

func TestRerunConfirmInvokesCmd(t *testing.T) {
	m := failingCIModel(t)
	m.modal = modalRerun
	m.rerunRun = m.workflows[0].Runs[1]
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should return a rerun cmd")
	}
	if m.modal != modalNone || !m.rerunning {
		t.Fatalf("after confirm: modal=%v rerunning=%v", m.modal, m.rerunning)
	}
}

func TestRerunDoneToast(t *testing.T) {
	m := failingCIModel(t)
	m.rerunning = true
	m.Update(rerunDoneMsg{run: ci.Run{RunNumber: 2}})
	if m.rerunning || m.toast == "" {
		t.Fatalf("rerun done not handled: rerunning=%v toast=%q", m.rerunning, m.toast)
	}
}

func TestDebugInertWhenNotEligible(t *testing.T) {
	m := failingCIModel(t)
	m.cursor = 2
	// not in cmux -> ineligible; d should be inert (no cmd, no panic)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd != nil {
		t.Fatal("d should be inert when not eligible")
	}
}

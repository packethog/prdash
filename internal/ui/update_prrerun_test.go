package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/packethog/prdash/internal/pr"
)

// uniquely named to avoid colliding with any existing test error in the package.
var errPRRerun = errors.New("boom")

func prRerunPR(num int, rollup string) pr.PR {
	return pr.PR{Repo: "o/r", Number: num, URL: "u", HeadRefName: "feat", HeadSHA: "abc", RollupState: rollup}
}

func keyR() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}} }

func TestPRRerunArmsModalOnFailedAuthored(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.section = secAuthored
	m.authored = []pr.PR{prRerunPR(1, "FAILURE")}
	m.cursor = 0
	m.Update(keyR())
	if m.modal != modalPRRerun || m.modalPR.Number != 1 {
		t.Fatalf("R should arm modalPRRerun: modal=%v pr=%d", m.modal, m.modalPR.Number)
	}
}

func TestPRRerunInertCases(t *testing.T) {
	cases := []struct {
		name    string
		section section
		rollup  string
	}{
		{"success", secAuthored, "SUCCESS"},
		{"pending", secAuthored, "PENDING"},
		{"none", secAuthored, ""},
		{"reviewing bucket", secReviewing, "FAILURE"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := New(stubRunner{}, time.Second, 10)
			m.section = c.section
			p := prRerunPR(1, c.rollup)
			if c.section == secReviewing {
				m.reviewing = []pr.PR{p}
			} else {
				m.authored = []pr.PR{p}
			}
			m.cursor = 0
			m.Update(keyR())
			if m.modal == modalPRRerun {
				t.Fatalf("%s: R should not arm the rerun modal", c.name)
			}
		})
	}
}

func TestPRRerunConfirmDispatches(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.section = secAuthored
	m.authored = []pr.PR{prRerunPR(1, "FAILURE")}
	m.modal = modalPRRerun
	m.modalPR = m.authored[0]
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should dispatch the rerun cmd")
	}
	if m.modal != modalNone || !m.prRerunning {
		t.Fatalf("after confirm: modal=%v prRerunning=%v", m.modal, m.prRerunning)
	}
}

func TestPRRerunEscCancels(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.modal = modalPRRerun
	m.modalPR = prRerunPR(1, "FAILURE")
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.modal != modalNone {
		t.Fatal("esc should cancel")
	}
}

func TestPRRerunDoneToasts(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.prRerunning = true
	m.Update(prRerunDoneMsg{p: prRerunPR(2, "FAILURE"), count: 3})
	if m.prRerunning || !strings.Contains(m.toast, "3") {
		t.Fatalf("count>0 toast wrong: rerunning=%v toast=%q", m.prRerunning, m.toast)
	}
	m.Update(prRerunDoneMsg{p: prRerunPR(2, "FAILURE"), count: 0})
	if !strings.Contains(strings.ToLower(m.toast), "no failed") {
		t.Fatalf("count==0 toast wrong: %q", m.toast)
	}
}

func TestPRRerunFailedToast(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.prRerunning = true
	m.Update(prRerunFailedMsg{p: prRerunPR(1, "FAILURE"), count: 0, err: errPRRerun})
	if m.prRerunning || !strings.Contains(m.toast, "Rerun failed") {
		t.Fatalf("failed toast wrong: %q", m.toast)
	}
	// partial-progress failure surfaces the count
	m.Update(prRerunFailedMsg{p: prRerunPR(1, "FAILURE"), count: 2, err: errPRRerun})
	if !strings.Contains(m.toast, "2") {
		t.Fatalf("partial-failure toast should mention the count: %q", m.toast)
	}
}

func TestPRRerunHintVisibility(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.width, m.height = 160, 40
	m.section = secAuthored
	m.authored = []pr.PR{prRerunPR(1, "FAILURE")}
	m.cursor = 0
	if out := m.View(); !strings.Contains(out, "R rerun") {
		t.Errorf("hint should show for authored failed-CI PR:\n%s", out)
	}
	m.authored = []pr.PR{prRerunPR(2, "SUCCESS")}
	if out := m.View(); strings.Contains(out, "R rerun") {
		t.Error("hint must be hidden when CI is not failed")
	}
}

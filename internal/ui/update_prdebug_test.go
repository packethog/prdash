package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/packethog/prdash/internal/config"
	"github.com/packethog/prdash/internal/pr"
)

func prDebugCfg(t *testing.T) config.PRDebug {
	t.Helper()
	d, err := config.ParsePRDebug("claude", "debug {{.URL}}")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func prWithRollup(num int, rollup string) pr.PR {
	return pr.PR{Repo: "o/r", Number: num, URL: "u", RollupState: rollup}
}

// d (space-less) is sent as a rune key.
func keyD() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}} }

func TestPRDebugDispatchesOnFailedAuthoredInCmux(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "1")
	m := New(stubRunner{}, time.Second, 10, WithPRDebug(prDebugCfg(t)))
	m.section = secAuthored
	m.authored = []pr.PR{prWithRollup(1, "FAILURE")}
	m.cursor = 0
	if _, cmd := m.Update(keyD()); cmd == nil {
		t.Fatal("d should dispatch on an authored failed-CI PR in cmux")
	}
}

// d is inert for every non-failed CI state, outside cmux, and outside Authored.
func TestPRDebugInertCases(t *testing.T) {
	cases := []struct {
		name    string
		cmux    string
		section section
		rollup  string
		cfg     bool
	}{
		{"ci success", "1", secAuthored, "SUCCESS", true},
		{"ci pending", "1", secAuthored, "PENDING", true},
		{"ci none", "1", secAuthored, "", true},
		{"outside cmux", "", secAuthored, "FAILURE", true},
		{"reviewing bucket", "1", secReviewing, "FAILURE", true},
		{"unconfigured", "1", secAuthored, "FAILURE", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("CMUX_WORKSPACE_ID", c.cmux)
			var opts []Option
			if c.cfg {
				opts = append(opts, WithPRDebug(prDebugCfg(t)))
			}
			m := New(stubRunner{}, time.Second, 10, opts...)
			m.section = c.section
			p := prWithRollup(1, c.rollup)
			if c.section == secReviewing {
				m.reviewing = []pr.PR{p}
			} else {
				m.authored = []pr.PR{p}
			}
			m.cursor = 0
			if _, cmd := m.Update(keyD()); cmd != nil {
				t.Fatalf("%s: d should be inert", c.name)
			}
		})
	}
}

func TestPRDebugCmdRendersAndDispatches(t *testing.T) {
	cmux := stubRunner{out: []byte("OK surface:4 pane:1 workspace:1")}
	msg := prDebugCmd(cmux, prDebugCfg(t), prWithRollup(1, "FAILURE"))()
	got, ok := msg.(prDebugLaunchedMsg)
	if !ok {
		t.Fatalf("want prDebugLaunchedMsg, got %T", msg)
	}
	if got.err != nil {
		t.Fatalf("dispatch err: %v", got.err)
	}
}

func TestPRDebugLaunchedToast(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10, WithPRDebug(prDebugCfg(t)))
	m.Update(prDebugLaunchedMsg{p: prWithRollup(1, "FAILURE")})
	if m.toast == "" {
		t.Fatal("expected a toast after dispatch")
	}
}

func TestPRDebugHintVisibility(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "1")
	m := New(stubRunner{}, time.Second, 10, WithPRDebug(prDebugCfg(t)))
	m.width, m.height = 160, 40
	m.section = secAuthored
	m.authored = []pr.PR{prWithRollup(1, "FAILURE")}
	m.cursor = 0
	if out := m.View(); !strings.Contains(out, "d debug") {
		t.Errorf("hint should show for an authored failed-CI PR in cmux:\n%s", out)
	}
	m.authored = []pr.PR{prWithRollup(2, "SUCCESS")}
	if out := m.View(); strings.Contains(out, "d debug") {
		t.Error("hint must be hidden when CI is not failed")
	}
}

package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/packethog/prdash/internal/ci"
	"github.com/packethog/prdash/internal/config"
	"github.com/packethog/prdash/internal/pr"
)

func testCIConfig(t *testing.T) config.CI {
	t.Helper()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "prdash"), 0o755)
	body := "ci:\n  limit: 5\n  provider: claude\n  prompt: \"debug {{.URL}}\"\n  workflows:\n    - repo: a/b\n      workflow: w.yml\n      name: QA\n      summaryArtifact: qa-analysis-*\n"
	_ = os.WriteFile(filepath.Join(dir, "prdash", "config.yaml"), []byte(body), 0o644)
	t.Setenv("XDG_CONFIG_HOME", dir)
	_, c, _, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func key(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func TestNavCursorClampsDown(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.authored = []pr.PR{{Number: 1}, {Number: 2}}
	m.Update(key("down"))
	m.Update(key("down")) // would go past end
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want clamped at 1", m.cursor)
	}
}

func TestNavCursorClampsUp(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.authored = []pr.PR{{Number: 1}}
	m.Update(key("up"))
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
}

func TestNavTabSwitchesSectionAndResetsCursor(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.authored = []pr.PR{{Number: 1}, {Number: 2}}
	m.reviewing = []pr.PR{{Number: 9}}
	m.cursor = 1
	m.Update(key("tab"))
	if m.section != secReviewing || m.cursor != 0 {
		t.Errorf("after tab: section=%v cursor=%d", m.section, m.cursor)
	}
}

func TestTabRotatesThroughCIWhenEnabled(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10, WithCI(testCIConfig(t)))
	m.workflows = []ci.WorkflowRuns{{Name: "QA", Key: "w.yml", Repo: "a/b"}}
	if m.section != secAuthored {
		t.Fatal("start at authored")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.section != secReviewing {
		t.Fatalf("after 1 tab: %v", m.section)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.section != secCI {
		t.Fatalf("after 2 tabs: %v", m.section)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.section != secAuthored {
		t.Fatalf("after 3 tabs: %v", m.section)
	}
}

func TestTabSkipsCIWhenDisabled(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10) // no CI config
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.section != secAuthored {
		t.Fatalf("should cycle only 2 sections, got %v", m.section)
	}
}

func TestCIFetchedMsgUpdatesWorkflows(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10, WithCI(testCIConfig(t)))
	wfs := []ci.WorkflowRuns{{Name: "QA", Runs: []ci.Run{{RunID: 1}}}}
	m.Update(ciFetchedMsg{workflows: wfs})
	if len(m.workflows) != 1 || len(m.workflows[0].Runs) != 1 {
		t.Fatalf("workflows not updated: %+v", m.workflows)
	}
}

func TestNavRefreshKeyStartsFetch(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.fetching = false
	_, cmd := m.Update(key("r"))
	if cmd == nil || !m.fetching {
		t.Error("r should start a fetch when idle")
	}
}

func TestNavQuit(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("ctrl+c should return a quit command")
	}
}

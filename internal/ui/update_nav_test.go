package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/packethog/prdash/internal/pr"
)

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

func TestNavTabSwitchesBucketAndResetsCursor(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.authored = []pr.PR{{Number: 1}, {Number: 2}}
	m.reviewing = []pr.PR{{Number: 9}}
	m.cursor = 1
	m.Update(key("tab"))
	if m.bucket != pr.AwaitingReview || m.cursor != 0 {
		t.Errorf("after tab: bucket=%v cursor=%d", m.bucket, m.cursor)
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

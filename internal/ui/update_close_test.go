package ui

import (
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/packethog/prdash/internal/gh"
	"github.com/packethog/prdash/internal/pr"
)

func authoredPR() pr.PR {
	return pr.PR{Repo: "o/r", Number: 7, URL: "u", Title: "t"}
}

func TestCloseDoneMarksActioned(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.Update(closeDoneMsg{p: pr.PR{Repo: "o/r", Number: 1, URL: "u1"}})
	if !m.actioned["u1"] {
		t.Error("closeDone should mark the PR struck (actioned) until the refetch drops it")
	}
}

func TestActionedPrunedWhenRowGone(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.markActioned("u1")
	m.Update(prsFetchedMsg{res: gh.FetchResult{Authored: []pr.PR{{URL: "u2"}}}})
	if m.actioned["u1"] {
		t.Error("an actioned PR no longer listed should be pruned")
	}
}

func TestActionedKeptWhileStillListed(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.markActioned("u1")
	m.Update(prsFetchedMsg{res: gh.FetchResult{Authored: []pr.PR{{URL: "u1"}}}})
	if !m.actioned["u1"] {
		t.Error("an actioned PR still listed should stay struck")
	}
}

func TestModalDismissedWhenCapturedPRVanishes(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.conn = connLive
	m.authored = []pr.PR{authoredPR()}
	m.Update(key("c"))
	if m.modal != modalClose {
		t.Fatal("c should arm the close modal")
	}
	// a refetch no longer lists the captured PR (closed/merged elsewhere)
	m.Update(prsFetchedMsg{res: gh.FetchResult{Authored: []pr.PR{{Repo: "o/r", Number: 9, URL: "other"}}}})
	if m.modal != modalNone {
		t.Error("modal should dismiss when the captured PR is no longer listed")
	}
}

func TestCKeyOpensCloseModalOnAuthored(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.authored = []pr.PR{authoredPR()}
	m.Update(key("c"))
	if m.modal != modalClose {
		t.Error("c on an authored PR should open the close modal")
	}
	if m.modalPR.Number != 7 {
		t.Error("c should capture the selected PR")
	}
}

func TestCKeyIgnoredOnReviewingBucket(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.reviewing = []pr.PR{authoredPR()}
	m.section = secReviewing
	m.Update(key("c"))
	if m.modal != modalNone {
		t.Error("c on the reviewing bucket should not open the close modal")
	}
}

func TestCloseModalEnterClosesWhenLive(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.conn = connLive
	m.authored = []pr.PR{authoredPR()}
	m.Update(key("c"))
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("enter should issue a close command when live")
	}
	if !m.closing || m.modal != modalNone {
		t.Errorf("after confirm: closing=%v modal=%v", m.closing, m.modal)
	}
}

func TestCloseModalEnterBlockedWhenNotLive(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.conn = connOffline // New default; close requires Live
	m.authored = []pr.PR{authoredPR()}
	m.Update(key("c"))
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || m.closing {
		t.Error("enter must do nothing when the connection is not live")
	}
	if m.modal != modalClose {
		t.Error("blocked close modal should stay open")
	}
}

func TestCloseModalEscCloses(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.conn = connLive
	m.authored = []pr.PR{authoredPR()}
	m.Update(key("c"))
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.modal != modalNone {
		t.Error("esc should close the modal")
	}
}

func TestCloseDoneTriggersRefetch(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.closing = true
	m.fetching = false
	_, cmd := m.Update(closeDoneMsg{p: authoredPR()})
	if m.closing {
		t.Error("closing should clear on done")
	}
	if m.toast == "" {
		t.Error("expected a success toast")
	}
	if cmd == nil {
		t.Error("close done should refetch")
	}
}

func TestCloseFailedShowsToastKeepsList(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.authored = []pr.PR{authoredPR()}
	m.closing = true
	m.Update(closeFailedMsg{p: authoredPR(), err: errors.New("boom")})
	if m.closing {
		t.Error("closing should clear on failure")
	}
	if m.toast == "" {
		t.Error("expected a failure toast")
	}
	if len(m.authored) != 1 {
		t.Error("list must be retained on close failure")
	}
}

package ui

import (
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/packethog/prdash/internal/gh"
	"github.com/packethog/prdash/internal/pr"
)

func mergeablePR() pr.PR {
	return pr.PR{Repo: "o/r", Number: 1, URL: "u", ReviewDecision: "APPROVED", RollupState: "SUCCESS", Mergeable: "MERGEABLE"}
}

func TestMKeyOpensModalOnAuthored(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.authored = []pr.PR{mergeablePR()}
	m.Update(key("m"))
	if m.modal != modalMerge {
		t.Error("m on authored should open the merge modal")
	}
}

func TestMKeyIgnoredOnReviewingBucket(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.reviewing = []pr.PR{mergeablePR()}
	m.bucket = pr.AwaitingReview
	m.Update(key("m"))
	if m.modal != modalNone {
		t.Error("m on the reviewing bucket should not open the modal")
	}
}

func TestModalEnterMergesWhenArmed(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.conn = connLive
	m.authored = []pr.PR{mergeablePR()}
	m.Update(key("m"))
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("enter on an armed PR should issue a merge command")
	}
	if !m.merging || m.modal != modalNone {
		t.Errorf("after confirm: merging=%v modal=%v", m.merging, m.modal)
	}
}

func TestModalEnterBlockedWhenGateFails(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.authored = []pr.PR{{Repo: "o/r", Number: 2, URL: "u", ReviewDecision: "", RollupState: "PENDING"}}
	m.Update(key("m"))
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || m.merging {
		t.Error("enter must do nothing when the gate fails")
	}
	if m.modal != modalMerge {
		t.Error("blocked modal should stay open")
	}
}

func TestModalCyclesMethod(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.authored = []pr.PR{mergeablePR()}
	m.Update(key("m"))
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.method != pr.MethodMerge {
		t.Errorf("right should advance method, got %v", m.method)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.method != pr.MethodSquash {
		t.Errorf("left should go back, got %v", m.method)
	}
}

func TestModalEscCloses(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.authored = []pr.PR{mergeablePR()}
	m.Update(key("m"))
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.modal != modalNone {
		t.Error("esc should close the modal")
	}
}

func TestMergeDoneTriggersRefetch(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.merging = true
	m.fetching = false
	_, cmd := m.Update(mergeDoneMsg{p: pr.PR{Repo: "o/r", Number: 1}})
	if m.merging {
		t.Error("merging should clear")
	}
	if m.toast == "" {
		t.Error("expected a success toast")
	}
	if cmd == nil {
		t.Error("merge done should refetch")
	}
}

func TestMergeFailedShowsToastKeepsList(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.authored = []pr.PR{mergeablePR()}
	m.merging = true
	m.Update(mergeFailedMsg{p: mergeablePR(), err: errors.New("blocked")})
	if m.merging {
		t.Error("merging should clear")
	}
	if m.toast == "" {
		t.Error("expected a failure toast")
	}
	if len(m.authored) != 1 {
		t.Error("list must be retained on merge failure")
	}
}

func TestOpenKeyIssuesCommand(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.authored = []pr.PR{mergeablePR()}
	_, cmd := m.Update(key("o"))
	if cmd == nil {
		t.Error("o should issue an open command")
	}
}

func TestModalEnterBlockedWhenOffline(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	// conn is connOffline by default from New; modal PR is fully mergeable
	m.authored = []pr.PR{mergeablePR()}
	m.Update(key("m"))
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("enter should not merge when connection is not live")
	}
	if m.merging {
		t.Error("merging should not be set when blocked by offline connection")
	}
	if m.modal != modalMerge {
		t.Error("modal should remain open when blocked")
	}
}

func TestMergeDoneWhileFetchingDefersRefresh(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.fetching = true
	m.Update(mergeDoneMsg{p: pr.PR{Repo: "o/r", Number: 1}})
	if !m.pendingRefresh {
		t.Error("expected pendingRefresh when merge done while fetching")
	}
	if m.merging {
		t.Error("merging should clear on merge done")
	}
	_, cmd := m.Update(prsFetchedMsg{res: gh.FetchResult{}})
	if cmd == nil {
		t.Error("expected a fetch command after draining pendingRefresh")
	}
	if m.pendingRefresh {
		t.Error("pendingRefresh should be cleared after drain")
	}
	if !m.fetching {
		t.Error("fetching should be set when draining pendingRefresh")
	}
}

func TestMergeDoneDefersRefreshDrainsOnFailedFetch(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.fetching = true
	m.Update(mergeDoneMsg{p: pr.PR{Repo: "o/r", Number: 1}})
	if !m.pendingRefresh {
		t.Fatal("expected pendingRefresh when merge done while fetching")
	}
	// The in-flight fetch fails; the deferred refresh must still drain.
	_, cmd := m.Update(fetchFailedMsg{err: errors.New("offline")})
	if cmd == nil {
		t.Error("expected a fetch command after draining pendingRefresh on failed fetch")
	}
	if m.pendingRefresh {
		t.Error("pendingRefresh should be cleared after drain")
	}
	if !m.fetching {
		t.Error("fetching should be set when draining pendingRefresh")
	}
}

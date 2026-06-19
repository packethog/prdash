package ui

import (
	"errors"
	"testing"
	"time"

	"github.com/packethog/prdash/internal/gh"
	"github.com/packethog/prdash/internal/pr"
)

func fixedClock(ts time.Time) func() time.Time { return func() time.Time { return ts } }

func TestUpdateFetchedGoesLive(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	ts := time.Date(2026, 6, 18, 15, 0, 0, 0, time.UTC)
	m.now = fixedClock(ts)

	res := gh.FetchResult{Authored: []pr.PR{{Number: 1}}, Reviewing: []pr.PR{{Number: 2}}}
	model, cmd := m.Update(prsFetchedMsg{res: res})
	m = model.(*Model)

	if m.conn != connLive {
		t.Errorf("conn = %v, want Live", m.conn)
	}
	if m.fetching {
		t.Error("fetching should be cleared after a result")
	}
	if !m.lastUpdated.Equal(ts) {
		t.Error("lastUpdated should be set from the clock")
	}
	if m.backoff.Failures() != 0 {
		t.Error("success should reset backoff")
	}
	if cmd == nil {
		t.Error("a follow-up tick command is expected")
	}
}

func TestUpdateFirstFailureWithDataIsStale(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	m.authored = []pr.PR{{Number: 1}} // have cached data
	m.fetching = true

	model, _ := m.Update(fetchFailedMsg{err: errors.New("offline")})
	m = model.(*Model)

	if m.conn != connStale {
		t.Errorf("first failure with data: conn = %v, want Stale", m.conn)
	}
	if len(m.authored) != 1 {
		t.Error("cached data must be retained on failure")
	}
}

func TestUpdateSecondFailureIsOffline(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	m.authored = []pr.PR{{Number: 1}}
	m.Update(fetchFailedMsg{err: errors.New("e1")})
	model, _ := m.Update(fetchFailedMsg{err: errors.New("e2")})
	m = model.(*Model)
	if m.conn != connOffline {
		t.Errorf("second failure: conn = %v, want Offline", m.conn)
	}
}

func TestUpdateFailureWithNoDataIsOffline(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	model, _ := m.Update(fetchFailedMsg{err: errors.New("offline")})
	m = model.(*Model)
	if m.conn != connOffline {
		t.Errorf("no-data failure: conn = %v, want Offline", m.conn)
	}
}

func TestTickTriggersFetchWhenIdle(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	m.fetching = false
	_, cmd := m.Update(tickMsg{})
	if cmd == nil {
		t.Error("idle tick should start a fetch")
	}
	if !m.fetching {
		t.Error("tick should set fetching")
	}
}

func TestTickIgnoredWhileFetching(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	m.fetching = true
	if _, cmd := m.Update(tickMsg{}); cmd != nil {
		t.Error("tick while fetching should be a no-op")
	}
}

func TestStaleTickIgnored(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	m.tickGen = 3
	m.fetching = false
	_, cmd := m.Update(tickMsg{gen: 1})
	if cmd != nil {
		t.Error("stale tick should not produce a command")
	}
	if m.fetching {
		t.Error("stale tick should not set fetching")
	}
}

func TestUITickReschedulesWithoutFetching(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	m.fetching = false
	m.tickGen = 7
	_, cmd := m.Update(uiTickMsg{})
	if cmd == nil {
		t.Error("uiTick should reschedule the heartbeat")
	}
	if m.fetching {
		t.Error("uiTick must not start a fetch")
	}
	if m.tickGen != 7 {
		t.Error("uiTick must not touch the data-refresh generation")
	}
}

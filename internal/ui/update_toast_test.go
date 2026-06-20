package ui

import (
	"testing"
	"time"

	"github.com/packethog/prdash/internal/pr"
)

// TestToastExpiresAfterTTL verifies a status toast is held until toastTTL and
// then cleared by the 1s UI tick, instead of lingering forever.
func TestToastExpiresAfterTTL(t *testing.T) {
	base := time.Unix(1000, 0)
	now := base
	m := New(stubRunner{}, time.Second, 10)
	m.now = func() time.Time { return now }

	m.Update(reviewLaunchedMsg{p: pr.PR{Repo: "o/r", Number: 1}})
	if m.toast == "" {
		t.Fatal("expected a toast to be set")
	}

	// A tick just before the TTL keeps the toast.
	now = base.Add(toastTTL - time.Millisecond)
	m.Update(uiTickMsg{})
	if m.toast == "" {
		t.Error("toast cleared before toastTTL")
	}

	// A tick at/after the TTL clears it.
	now = base.Add(toastTTL)
	m.Update(uiTickMsg{})
	if m.toast != "" {
		t.Errorf("toast should have expired, got %q", m.toast)
	}
}

// TestToastTickResetsClockPerToast verifies the TTL is measured from when each
// toast was set, so a newer toast is not expired by an older toast's age.
func TestToastTickResetsClockPerToast(t *testing.T) {
	base := time.Unix(1000, 0)
	now := base
	m := New(stubRunner{}, time.Second, 10)
	m.now = func() time.Time { return now }

	m.Update(reviewLaunchedMsg{p: pr.PR{Repo: "o/r", Number: 1}})

	// Most of the first toast's life elapses, then a second toast is shown.
	now = base.Add(toastTTL - time.Second)
	m.Update(mergeDoneMsg{p: pr.PR{Repo: "o/r", Number: 2}})
	if m.toast != "Merged o/r#2" {
		t.Fatalf("expected the newer toast, got %q", m.toast)
	}

	// A tick past the FIRST toast's deadline must not clear the newer one.
	now = base.Add(toastTTL + time.Millisecond)
	m.Update(uiTickMsg{})
	if m.toast != "Merged o/r#2" {
		t.Errorf("newer toast expired on the older toast's clock, got %q", m.toast)
	}
}

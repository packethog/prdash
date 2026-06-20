package ui

import (
	"errors"
	"testing"
	"time"

	"github.com/packethog/prdash/internal/config"
	"github.com/packethog/prdash/internal/pr"
)

func TestReviewLaunchedSuccessToast(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.Update(reviewLaunchedMsg{p: pr.PR{Repo: "o/r", Number: 7}})
	if m.toast != "Review started for o/r#7" {
		t.Errorf("toast = %q", m.toast)
	}
}

func TestReviewLaunchedFailureToast(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.Update(reviewLaunchedMsg{p: pr.PR{Repo: "o/r", Number: 7}, err: errors.New("boom")})
	if m.toast != "Review failed: boom" {
		t.Errorf("toast = %q", m.toast)
	}
}

func enabledReview(t *testing.T) config.Review {
	t.Helper()
	r, err := config.Parse("claude", "review {{.URL}}")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestReviewKeyLaunchesWhenEligible(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "ws1")
	m := New(stubRunner{}, time.Second, 10, WithReview(enabledReview(t)))
	m.cmux = stubRunner{out: []byte("surface:4")}
	m.section = secReviewing
	m.reviewing = []pr.PR{{URL: "https://u", Repo: "o/r", Number: 7}}
	_, cmd := m.Update(key("v"))
	if cmd == nil {
		t.Fatal("v should return a launch command when eligible")
	}
}

func TestReviewKeyInertWhenNotInCmux(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "")
	m := New(stubRunner{}, time.Second, 10, WithReview(enabledReview(t)))
	m.section = secReviewing
	m.reviewing = []pr.PR{{URL: "https://u"}}
	if _, cmd := m.Update(key("v")); cmd != nil {
		t.Error("v must be inert outside cmux")
	}
}

func TestReviewKeyInertInAuthoredBucket(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "ws1")
	m := New(stubRunner{}, time.Second, 10, WithReview(enabledReview(t)))
	m.section = secAuthored
	m.authored = []pr.PR{{URL: "https://u"}}
	if _, cmd := m.Update(key("v")); cmd != nil {
		t.Error("v must be inert in the Authored section")
	}
}

func TestReviewKeyInertWhenUnconfigured(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "ws1")
	m := New(stubRunner{}, time.Second, 10) // no WithReview
	m.section = secReviewing
	m.reviewing = []pr.PR{{URL: "https://u"}}
	if _, cmd := m.Update(key("v")); cmd != nil {
		t.Error("v must be inert when review is unconfigured")
	}
}

package ui

import (
	"errors"
	"testing"
	"time"

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

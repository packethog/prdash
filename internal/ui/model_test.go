package ui

import (
	"testing"
	"time"

	"github.com/packethog/prdash/internal/pr"
)

func TestNewDefaults(t *testing.T) {
	m := New(stubRunner{}, 45*time.Second, 50)
	if !m.fetching {
		t.Error("New should start in fetching state")
	}
	if m.bucket != pr.Authored {
		t.Error("New should start on the Authored bucket")
	}
	if m.method != pr.MethodSquash {
		t.Error("default method should be squash")
	}
	if m.Init() == nil {
		t.Error("Init should return a command")
	}
}

func TestRowsAndSelected(t *testing.T) {
	m := New(stubRunner{}, time.Second, 10)
	m.authored = []pr.PR{{Number: 1}, {Number: 2}}
	m.reviewing = []pr.PR{{Number: 9}}

	if len(m.rows()) != 2 {
		t.Errorf("authored rows = %d", len(m.rows()))
	}
	m.cursor = 1
	if p, ok := m.selected(); !ok || p.Number != 2 {
		t.Errorf("selected = %+v ok=%v", p, ok)
	}
	m.bucket = pr.AwaitingReview
	m.cursor = 0
	if p, ok := m.selected(); !ok || p.Number != 9 {
		t.Errorf("reviewing selected = %+v ok=%v", p, ok)
	}
	m.cursor = 5
	if _, ok := m.selected(); ok {
		t.Error("out-of-range cursor should not select")
	}
}

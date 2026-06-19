package gh

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSurfaceArgs(t *testing.T) {
	got := strings.Join(surfaceArgs("claude"), " ")
	want := "new-surface --type agent-session --provider claude --focus true"
	if got != want {
		t.Errorf("surfaceArgs = %q, want %q", got, want)
	}
}

func TestParseSurfaceRef(t *testing.T) {
	ref, err := parseSurfaceRef([]byte("surface:4\n"))
	if err != nil || ref != "surface:4" {
		t.Errorf("ref=%q err=%v", ref, err)
	}
	if _, err = parseSurfaceRef([]byte("  \n")); err == nil {
		t.Error("expected error on empty output")
	}
	ref, err = parseSurfaceRef([]byte("warning: deprecated flag\nsurface:7\n"))
	if err != nil || ref != "surface:7" {
		t.Errorf("noisy output: ref=%q err=%v", ref, err)
	}
}

func TestStartReviewDrivesCmux(t *testing.T) {
	f := &fakeRunner{out: []byte("surface:4\n")}
	if err := StartReview(context.Background(), f, "claude", "review https://u"); err != nil {
		t.Fatal(err)
	}
	if len(f.gotArgs) != 3 {
		t.Fatalf("want 3 cmux calls, got %d: %v", len(f.gotArgs), f.gotArgs)
	}
	if strings.Join(f.gotArgs[0], " ") != "new-surface --type agent-session --provider claude --focus true" {
		t.Errorf("call 0 = %v", f.gotArgs[0])
	}
	if strings.Join(f.gotArgs[1], " ") != "send --surface surface:4 -- review https://u" {
		t.Errorf("call 1 = %v", f.gotArgs[1])
	}
	if strings.Join(f.gotArgs[2], " ") != "send-key --surface surface:4 enter" {
		t.Errorf("call 2 = %v", f.gotArgs[2])
	}
}

func TestStartReviewPropagatesSpawnError(t *testing.T) {
	f := &fakeRunner{err: errors.New("cmux not found")}
	if err := StartReview(context.Background(), f, "claude", "x"); err == nil {
		t.Fatal("expected error when new-surface fails")
	}
}

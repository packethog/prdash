package gh

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestPaneArgs(t *testing.T) {
	got := strings.Join(paneArgs(), " ")
	want := "new-pane --type terminal --direction down --focus true"
	if got != want {
		t.Errorf("paneArgs = %q, want %q", got, want)
	}
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote("a b"); got != "'a b'" {
		t.Errorf("shellQuote(a b) = %q", got)
	}
	// An embedded single quote is closed, escaped, and reopened.
	if got := shellQuote("it's"); got != `'it'\''s'` {
		t.Errorf("shellQuote(it's) = %q", got)
	}
}

func TestParseSurfaceRef(t *testing.T) {
	// new-pane prints "OK surface:N pane:N workspace:N" — pick the surface token.
	ref, err := parseSurfaceRef([]byte("OK surface:32 pane:28 workspace:2\n"))
	if err != nil || ref != "surface:32" {
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

func TestReviewCommand(t *testing.T) {
	if got := reviewCommand("claude", nil, "review https://u"); got != "'claude' 'review https://u'" {
		t.Errorf("no args: %q", got)
	}
	got := reviewCommand("claude", []string{"--permission-mode", "auto"}, "go")
	if got != "'claude' '--permission-mode' 'auto' 'go'" {
		t.Errorf("with args: %q", got)
	}
}

func TestStartReviewDrivesCmux(t *testing.T) {
	f := &fakeRunner{out: []byte("OK surface:4 pane:3 workspace:2\n")}
	if err := StartReview(context.Background(), f, "claude", []string{"--permission-mode", "auto"}, "review https://u"); err != nil {
		t.Fatal(err)
	}
	if len(f.gotArgs) != 3 {
		t.Fatalf("want 3 cmux calls, got %d: %v", len(f.gotArgs), f.gotArgs)
	}
	if strings.Join(f.gotArgs[0], " ") != "new-pane --type terminal --direction down --focus true" {
		t.Errorf("call 0 = %v", f.gotArgs[0])
	}
	if strings.Join(f.gotArgs[1], " ") != "send --surface surface:4 -- 'claude' '--permission-mode' 'auto' 'review https://u'" {
		t.Errorf("call 1 = %v", f.gotArgs[1])
	}
	if strings.Join(f.gotArgs[2], " ") != "send-key --surface surface:4 enter" {
		t.Errorf("call 2 = %v", f.gotArgs[2])
	}
}

func TestStartReviewPropagatesSpawnError(t *testing.T) {
	f := &fakeRunner{err: errors.New("cmux not found")}
	if err := StartReview(context.Background(), f, "claude", nil, "x"); err == nil {
		t.Fatal("expected error when new-pane fails")
	}
}

func TestStartAgent(t *testing.T) {
	f := &fakeRunner{out: []byte("OK surface:7 pane:3 workspace:1")}
	if err := StartAgent(context.Background(), f, "claude", []string{"--permission-mode", "auto"}, "debug it"); err != nil {
		t.Fatal(err)
	}
	if len(f.gotArgs) != 3 {
		t.Fatalf("want 3 cmux calls, got %d", len(f.gotArgs))
	}
	send := f.gotArgs[1]
	if cmd := send[len(send)-1]; cmd != `'claude' '--permission-mode' 'auto' 'debug it'` {
		t.Errorf("command = %q", cmd)
	}
}

func TestStartCIDebug(t *testing.T) {
	f := &fakeRunner{out: []byte("OK surface:7 pane:3 workspace:1\n")}
	err := StartCIDebug(context.Background(), f, "claude", []string{"--permission-mode", "auto"}, "debug it")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.gotArgs) != 3 {
		t.Fatalf("want 3 cmux calls, got %d: %v", len(f.gotArgs), f.gotArgs)
	}
	if strings.Join(f.gotArgs[0], " ") != "new-pane --type terminal --direction down --focus true" {
		t.Errorf("call 0 = %v", f.gotArgs[0])
	}
	send := f.gotArgs[1]
	cmd := send[len(send)-1]
	if cmd != `'claude' '--permission-mode' 'auto' 'debug it'` {
		t.Errorf("command = %q", cmd)
	}
	if strings.Join(f.gotArgs[2], " ") != "send-key --surface surface:7 enter" {
		t.Errorf("call 2 = %v", f.gotArgs[2])
	}
}

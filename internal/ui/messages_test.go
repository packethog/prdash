package ui

import (
	"context"
	"errors"
	"testing"

	"github.com/packethog/prdash/internal/config"
	"github.com/packethog/prdash/internal/pr"
)

type stubRunner struct {
	out []byte
	err error
}

func (s stubRunner) Run(_ context.Context, _ ...string) ([]byte, error) { return s.out, s.err }

func TestFetchCmdSuccess(t *testing.T) {
	r := stubRunner{out: []byte(`{"data":{"authored":{"nodes":[{"number":1,"url":"u","repository":{"nameWithOwner":"o/r"}}]},"reviewing":{"nodes":[]}}}`)}
	msg := fetchCmd(r, 50)()
	got, ok := msg.(prsFetchedMsg)
	if !ok {
		t.Fatalf("expected prsFetchedMsg, got %T", msg)
	}
	if len(got.res.Authored) != 1 {
		t.Errorf("authored len = %d", len(got.res.Authored))
	}
}

func TestFetchCmdFailure(t *testing.T) {
	r := stubRunner{err: errors.New("offline")}
	if _, ok := fetchCmd(r, 50)().(fetchFailedMsg); !ok {
		t.Fatal("expected fetchFailedMsg")
	}
}

func TestMergeCmd(t *testing.T) {
	if _, ok := mergeCmd(stubRunner{}, pr.PR{URL: "u"}, pr.MethodSquash)().(mergeDoneMsg); !ok {
		t.Fatal("expected mergeDoneMsg on success")
	}
	if _, ok := mergeCmd(stubRunner{err: errors.New("x")}, pr.PR{URL: "u"}, pr.MethodSquash)().(mergeFailedMsg); !ok {
		t.Fatal("expected mergeFailedMsg on error")
	}
}

func TestOpenCmd(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "") // force the stubbed gh path; never exec real cmux
	msg := openCmd(stubRunner{}, pr.PR{URL: "u"})()
	if _, ok := msg.(openedMsg); !ok {
		t.Fatalf("expected openedMsg, got %T", msg)
	}
}

func TestCloseCmd(t *testing.T) {
	if _, ok := closeCmd(stubRunner{}, pr.PR{URL: "u"})().(closeDoneMsg); !ok {
		t.Fatal("expected closeDoneMsg on success")
	}
	if _, ok := closeCmd(stubRunner{err: errors.New("x")}, pr.PR{URL: "u"})().(closeFailedMsg); !ok {
		t.Fatal("expected closeFailedMsg on error")
	}
}

func TestReviewCmdLaunches(t *testing.T) {
	rv, err := config.Parse("claude", "review {{.URL}}")
	if err != nil {
		t.Fatal(err)
	}
	msg := reviewCmd(stubRunner{out: []byte("surface:4")}, rv, pr.PR{URL: "https://u"})()
	r, ok := msg.(reviewLaunchedMsg)
	if !ok || r.err != nil {
		t.Fatalf("msg = %#v", msg)
	}
}

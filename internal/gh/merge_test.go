package gh

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/packethog/prdash/internal/pr"
)

func TestMergeBuildsArgs(t *testing.T) {
	cases := []struct {
		method   pr.MergeMethod
		wantFlag string
	}{
		{pr.MethodSquash, "--squash"},
		{pr.MethodMerge, "--merge"},
		{pr.MethodRebase, "--rebase"},
	}
	for _, c := range cases {
		f := &fakeRunner{}
		p := pr.PR{URL: "https://github.com/o/r/pull/9"}
		if err := Merge(context.Background(), f, p, c.method, true); err != nil {
			t.Fatal(err)
		}
		args := strings.Join(f.gotArgs[0], " ")
		for _, want := range []string{"pr merge", p.URL, c.wantFlag, "--delete-branch"} {
			if !strings.Contains(args, want) {
				t.Errorf("method %v: args %q missing %q", c.method, args, want)
			}
		}
	}
}

func TestMergeNoDeleteBranch(t *testing.T) {
	f := &fakeRunner{}
	if err := Merge(context.Background(), f, pr.PR{URL: "u"}, pr.MethodSquash, false); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(f.gotArgs[0], " "), "--delete-branch") {
		t.Error("did not expect --delete-branch")
	}
}

func TestMergePropagatesError(t *testing.T) {
	f := &fakeRunner{err: errors.New("merge blocked")}
	if err := Merge(context.Background(), f, pr.PR{URL: "u"}, pr.MethodSquash, true); err == nil {
		t.Fatal("expected error")
	}
}

func TestOpenBuildsArgs(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "") // force the gh path (not the cmux pane path)
	f := &fakeRunner{}
	p := pr.PR{URL: "https://github.com/o/r/pull/9"}
	if err := Open(context.Background(), f, p); err != nil {
		t.Fatal(err)
	}
	args := strings.Join(f.gotArgs[0], " ")
	for _, want := range []string{"pr view", p.URL, "--web"} {
		if !strings.Contains(args, want) {
			t.Errorf("args %q missing %q", args, want)
		}
	}
}

func TestOpenArgs(t *testing.T) {
	bin, args := openArgs(false, "u")
	if bin != "gh" || strings.Join(args, " ") != "pr view u --web" {
		t.Errorf("non-cmux openArgs = %s %v", bin, args)
	}
	bin, args = openArgs(true, "u")
	if bin != "cmux" || strings.Join(args, " ") != "new-pane --type browser --direction down --url u" {
		t.Errorf("cmux openArgs = %s %v", bin, args)
	}
}

func TestCloseBuildsArgs(t *testing.T) {
	f := &fakeRunner{}
	p := pr.PR{URL: "https://github.com/o/r/pull/9"}
	if err := Close(context.Background(), f, p); err != nil {
		t.Fatal(err)
	}
	args := strings.Join(f.gotArgs[0], " ")
	for _, want := range []string{"pr close", p.URL} {
		if !strings.Contains(args, want) {
			t.Errorf("args %q missing %q", args, want)
		}
	}
	if strings.Contains(args, "--delete-branch") {
		t.Error("close must not delete the branch")
	}
}

func TestClosePropagatesError(t *testing.T) {
	f := &fakeRunner{err: errors.New("nope")}
	if err := Close(context.Background(), f, pr.PR{URL: "u"}); err == nil {
		t.Fatal("expected error")
	}
}

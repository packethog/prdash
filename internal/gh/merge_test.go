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

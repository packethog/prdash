package gh

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/packethog/prdash/internal/pr"
)

func TestFetchDecodesFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/search.json")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeRunner{out: data}

	res, err := Fetch(context.Background(), f, 50)
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}

	if len(res.Authored) != 2 {
		t.Fatalf("authored len = %d, want 2", len(res.Authored))
	}
	// #1234 is in both buckets; dedupe drops it from reviewing.
	if len(res.Reviewing) != 2 {
		t.Fatalf("reviewing len = %d, want 2 (after dedupe)", len(res.Reviewing))
	}

	p := res.Authored[0]
	if p.Repo != "malbeclabs/doublezero" || p.Number != 1234 {
		t.Errorf("authored[0] ref = %s", p.Ref())
	}
	if p.ReviewDecision != "APPROVED" || p.RollupState != "SUCCESS" {
		t.Errorf("authored[0] decision/rollup = %q/%q", p.ReviewDecision, p.RollupState)
	}
	if p.MergeStateStatus != "CLEAN" {
		t.Errorf("authored[0] mergeStateStatus = %q, want CLEAN", p.MergeStateStatus)
	}
	if !p.UpdatedAt.Equal(time.Date(2026, 6, 18, 15, 3, 9, 0, time.UTC)) {
		t.Errorf("authored[0] updatedAt = %v", p.UpdatedAt)
	}
	if res.Authored[1].ReviewDecision != "" || res.Authored[1].RollupState != "PENDING" {
		t.Errorf("null reviewDecision should decode to empty; got %q", res.Authored[1].ReviewDecision)
	}

	// The draft with a null rollup.
	var draft *pr.PR
	for i := range res.Reviewing {
		if res.Reviewing[i].Number == 7 {
			draft = &res.Reviewing[i]
		}
	}
	if draft == nil {
		t.Fatal("expected #7 in reviewing")
	}
	if !draft.IsDraft || draft.RollupState != "" || draft.Mergeable != "UNKNOWN" {
		t.Errorf("#7 decoded wrong: %+v", *draft)
	}
}

func TestFetchPassesQueryArgs(t *testing.T) {
	f := &fakeRunner{out: []byte(`{"data":{"authored":{"nodes":[]},"reviewing":{"nodes":[]}}}`)}
	if _, err := Fetch(context.Background(), f, 25); err != nil {
		t.Fatal(err)
	}
	args := f.gotArgs[0]
	joined := strings.Join(args, " ")
	for _, want := range []string{"api", "graphql", "author:@me", "review-requested:@me", "sort:updated-desc", "first=25"} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q: %v", want, args)
		}
	}
}

func TestFetchPropagatesRunnerError(t *testing.T) {
	f := &fakeRunner{err: errors.New("boom")}
	if _, err := Fetch(context.Background(), f, 50); err == nil {
		t.Fatal("expected error from runner")
	}
}

func TestFetchSurfacesGraphQLErrors(t *testing.T) {
	f := &fakeRunner{out: []byte(`{"errors":[{"message":"bad scope"}]}`)}
	if _, err := Fetch(context.Background(), f, 50); err == nil {
		t.Fatal("expected error from graphql errors block")
	}
}

func TestFetchDecodesLatestReviews(t *testing.T) {
	data, err := os.ReadFile("testdata/search.json")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Fetch(context.Background(), &fakeRunner{out: data}, 50)
	if err != nil {
		t.Fatal(err)
	}
	// authored[1] is #88, which the fixture gives a COMMENTED review.
	if got := res.Authored[1].LatestReviews; len(got) != 1 || got[0] != "COMMENTED" {
		t.Errorf("authored[1].LatestReviews = %v, want [COMMENTED]", got)
	}
	// A node without latestReviews decodes to an empty slice.
	if len(res.Authored[0].LatestReviews) != 0 {
		t.Errorf("authored[0].LatestReviews = %v, want empty", res.Authored[0].LatestReviews)
	}
}

// TestFetchDecodesOpinionatedReviews covers the latestOpinionatedReviews wiring:
// an approval that GitHub surfaces only there (latestReviews empty) must reach
// the PR so the badge can read it.
func TestFetchDecodesOpinionatedReviews(t *testing.T) {
	body := `{"data":{"authored":{"nodes":[]},"reviewing":{"nodes":[
		{"number":112,"url":"https://github.com/o/r/pull/112","repository":{"nameWithOwner":"o/r"},
		 "reviewDecision":null,
		 "latestReviews":{"nodes":[]},
		 "latestOpinionatedReviews":{"nodes":[{"state":"APPROVED"}]}}
	]}}}`
	f := &fakeRunner{out: []byte(body)}
	res, err := Fetch(context.Background(), f, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Reviewing) != 1 {
		t.Fatalf("reviewing len = %d, want 1", len(res.Reviewing))
	}
	got := res.Reviewing[0]
	if len(got.LatestReviews) != 0 {
		t.Errorf("LatestReviews = %v, want empty", got.LatestReviews)
	}
	if len(got.OpinionatedReviews) != 1 || got.OpinionatedReviews[0] != "APPROVED" {
		t.Errorf("OpinionatedReviews = %v, want [APPROVED]", got.OpinionatedReviews)
	}
	if state := pr.Review(got); state != pr.ReviewApproved {
		t.Errorf("Review() = %v, want Approved", state)
	}
}

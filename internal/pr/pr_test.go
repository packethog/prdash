package pr

import "testing"

func TestReviewStateString(t *testing.T) {
	cases := map[ReviewState]string{
		ReviewApproved:         "Approved",
		ReviewChangesRequested: "Changes requested",
		ReviewPending:          "Pending review",
		ReviewDraft:            "Draft",
		ReviewCommented:        "Commented",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("ReviewState(%d).String() = %q, want %q", s, got, want)
		}
	}
}

func TestCIStateSymbol(t *testing.T) {
	cases := map[CIState]string{
		CISuccess: "✓",
		CIPending: "·",
		CIFailure: "✗",
		CINone:    "–",
	}
	for s, want := range cases {
		if got := s.Symbol(); got != want {
			t.Errorf("CIState(%d).Symbol() = %q, want %q", s, got, want)
		}
	}
}

func TestMergeMethodCycle(t *testing.T) {
	if MethodSquash.String() != "squash" || MethodMerge.String() != "merge" || MethodRebase.String() != "rebase" {
		t.Fatal("unexpected method strings")
	}
	if MethodSquash.Next() != MethodMerge || MethodMerge.Next() != MethodRebase || MethodRebase.Next() != MethodSquash {
		t.Error("Next() does not cycle squash->merge->rebase->squash")
	}
	if MethodSquash.Prev() != MethodRebase {
		t.Error("Prev() does not wrap")
	}
}

func TestPRRef(t *testing.T) {
	p := PR{Repo: "malbeclabs/doublezero", Number: 1234}
	if p.Ref() != "malbeclabs/doublezero#1234" {
		t.Errorf("Ref() = %q", p.Ref())
	}
}

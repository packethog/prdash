package pr

import "testing"

func TestReview(t *testing.T) {
	cases := []struct {
		name string
		p    PR
		want ReviewState
	}{
		{"draft wins over decision", PR{IsDraft: true, ReviewDecision: "APPROVED"}, ReviewDraft},
		{"approved", PR{ReviewDecision: "APPROVED"}, ReviewApproved},
		{"changes", PR{ReviewDecision: "CHANGES_REQUESTED"}, ReviewChangesRequested},
		{"review required", PR{ReviewDecision: "REVIEW_REQUIRED"}, ReviewPending},
		{"empty decision", PR{ReviewDecision: ""}, ReviewPending},
	}
	for _, c := range cases {
		if got := Review(c.p); got != c.want {
			t.Errorf("%s: Review() = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestReviewCommented(t *testing.T) {
	cases := []struct {
		name string
		p    PR
		want ReviewState
	}{
		{"commented only", PR{LatestReviews: []string{"COMMENTED"}}, ReviewCommented},
		{"approved-but-insufficient shows commented", PR{ReviewDecision: "", LatestReviews: []string{"APPROVED"}}, ReviewCommented},
		{"changes from reviews when decision empty", PR{ReviewDecision: "", LatestReviews: []string{"CHANGES_REQUESTED"}}, ReviewChangesRequested},
		{"decision approved wins over reviews", PR{ReviewDecision: "APPROVED", LatestReviews: []string{"COMMENTED"}}, ReviewApproved},
		{"decision changes wins", PR{ReviewDecision: "CHANGES_REQUESTED", LatestReviews: []string{"COMMENTED"}}, ReviewChangesRequested},
		{"draft wins over reviews", PR{IsDraft: true, LatestReviews: []string{"COMMENTED"}}, ReviewDraft},
		{"only pending/dismissed is not commented", PR{LatestReviews: []string{"PENDING", "DISMISSED"}}, ReviewPending},
		{"no reviews is pending", PR{LatestReviews: nil}, ReviewPending},
	}
	for _, c := range cases {
		if got := Review(c.p); got != c.want {
			t.Errorf("%s: Review() = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCI(t *testing.T) {
	cases := []struct {
		state string
		want  CIState
	}{
		{"SUCCESS", CISuccess},
		{"PENDING", CIPending},
		{"EXPECTED", CIPending},
		{"FAILURE", CIFailure},
		{"ERROR", CIFailure},
		{"", CINone},
		{"WEIRD", CINone},
	}
	for _, c := range cases {
		if got := CI(PR{RollupState: c.state}); got != c.want {
			t.Errorf("CI(%q) = %v, want %v", c.state, got, c.want)
		}
	}
}

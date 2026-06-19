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
		{"approved review (null decision) shows approved", PR{ReviewDecision: "", LatestReviews: []string{"APPROVED"}}, ReviewApproved},
		{"approved with comments shows approved", PR{ReviewDecision: "", LatestReviews: []string{"APPROVED", "COMMENTED"}}, ReviewApproved},
		{"changes-requested wins over an approval", PR{ReviewDecision: "", LatestReviews: []string{"APPROVED", "CHANGES_REQUESTED"}}, ReviewChangesRequested},
		{"changes from reviews when decision empty", PR{ReviewDecision: "", LatestReviews: []string{"CHANGES_REQUESTED"}}, ReviewChangesRequested},
		{"decision approved wins over reviews", PR{ReviewDecision: "APPROVED", LatestReviews: []string{"COMMENTED"}}, ReviewApproved},
		{"decision changes wins", PR{ReviewDecision: "CHANGES_REQUESTED", LatestReviews: []string{"COMMENTED"}}, ReviewChangesRequested},
		{"draft wins over reviews", PR{IsDraft: true, LatestReviews: []string{"COMMENTED"}}, ReviewDraft},
		{"only pending/dismissed is not commented", PR{LatestReviews: []string{"PENDING", "DISMISSED"}}, ReviewPending},
		{"no reviews is pending", PR{LatestReviews: nil}, ReviewPending},
		// GitHub drops the reviewer from latestReviews once approving clears the
		// review request; the approval survives only in latestOpinionatedReviews.
		{"opinionated approval with empty latestReviews shows approved", PR{ReviewDecision: "", LatestReviews: nil, OpinionatedReviews: []string{"APPROVED"}}, ReviewApproved},
		{"opinionated changes-requested with empty latestReviews", PR{ReviewDecision: "", LatestReviews: nil, OpinionatedReviews: []string{"CHANGES_REQUESTED"}}, ReviewChangesRequested},
		{"opinionated changes wins over latestReviews approval", PR{ReviewDecision: "", LatestReviews: []string{"APPROVED"}, OpinionatedReviews: []string{"CHANGES_REQUESTED"}}, ReviewChangesRequested},
		{"opinionated approval with latest comment shows approved", PR{ReviewDecision: "", LatestReviews: []string{"COMMENTED"}, OpinionatedReviews: []string{"APPROVED"}}, ReviewApproved},
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

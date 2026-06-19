package pr

// Review maps a PR to its displayed review state. Precedence: draft, then the
// authoritative reviewDecision, then change-requests seen in the per-reviewer
// reviews (covers repos with no required-reviewer rule where reviewDecision is
// null), then any substantive review (Commented), else Pending.
func Review(p PR) ReviewState {
	if p.IsDraft {
		return ReviewDraft
	}
	if p.ReviewDecision == "APPROVED" {
		return ReviewApproved
	}
	if p.ReviewDecision == "CHANGES_REQUESTED" || hasReviewState(p, "CHANGES_REQUESTED") {
		return ReviewChangesRequested
	}
	if hasSubstantiveReview(p) {
		return ReviewCommented
	}
	return ReviewPending
}

func hasReviewState(p PR, state string) bool {
	for _, s := range p.LatestReviews {
		if s == state {
			return true
		}
	}
	return false
}

// hasSubstantiveReview reports whether any reviewer left a comment, approval, or
// change request — ignoring PENDING (unsubmitted) and DISMISSED reviews.
func hasSubstantiveReview(p PR) bool {
	for _, s := range p.LatestReviews {
		switch s {
		case "COMMENTED", "APPROVED", "CHANGES_REQUESTED":
			return true
		}
	}
	return false
}

// CI maps a PR's status-check rollup to its displayed CI state. Anything that
// is not an explicit success/pending/failure is treated as none.
func CI(p PR) CIState {
	switch p.RollupState {
	case "SUCCESS":
		return CISuccess
	case "PENDING", "EXPECTED":
		return CIPending
	case "FAILURE", "ERROR":
		return CIFailure
	default:
		return CINone
	}
}

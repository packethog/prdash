package pr

// Review maps a PR to its displayed review state. Precedence: draft, then the
// authoritative reviewDecision, then per-reviewer review states (which cover
// repos with no required-reviewer rule, where reviewDecision is null even after
// an approval): change-requests, then approvals, then plain comments, else
// Pending. An approval outranks a comment, so an "approved with comments" PR
// shows Approved rather than Commented.
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
	if hasReviewState(p, "APPROVED") {
		return ReviewApproved
	}
	if hasReviewState(p, "COMMENTED") {
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

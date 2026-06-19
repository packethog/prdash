package pr

import "slices"

// Review maps a PR to its displayed review state. Precedence: draft, then the
// authoritative reviewDecision, then per-reviewer review states (which cover
// repos with no required-reviewer rule, where reviewDecision is null even after
// an approval): change-requests, then approvals, then plain comments, else
// Pending. An approval outranks a comment, so an "approved with comments" PR
// shows Approved rather than Commented.
//
// Approvals and change-requests are read from latestOpinionatedReviews as well
// as latestReviews: GitHub drops a reviewer from latestReviews once approving
// clears their review request, so an approved-but-unrequired PR would otherwise
// show as Pending.
func Review(p PR) ReviewState {
	if p.IsDraft {
		return ReviewDraft
	}
	if p.ReviewDecision == "APPROVED" {
		return ReviewApproved
	}
	if p.ReviewDecision == "CHANGES_REQUESTED" || hasOpinion(p, "CHANGES_REQUESTED") {
		return ReviewChangesRequested
	}
	if hasOpinion(p, "APPROVED") {
		return ReviewApproved
	}
	if slices.Contains(p.LatestReviews, "COMMENTED") {
		return ReviewCommented
	}
	return ReviewPending
}

// hasOpinion reports whether any latest opinionated review or latest review has
// the given state. Both are consulted because either set can carry the approval
// or change-request for a given PR.
func hasOpinion(p PR, state string) bool {
	return slices.Contains(p.OpinionatedReviews, state) || slices.Contains(p.LatestReviews, state)
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

package pr

import "slices"

// Review maps a PR to its displayed review state. The reviewDecision is
// authoritative when set: APPROVED and CHANGES_REQUESTED map straight through,
// and any other non-empty decision (e.g. REVIEW_REQUIRED) means the required
// reviews are NOT satisfied — so per-reviewer approvals must not upgrade it to
// Approved; it shows Commented if someone left a comment, else Pending.
//
// Only when reviewDecision is empty (repos with no required-reviewer rule, where
// it is null even after an approval) is the state derived from per-reviewer
// opinions: change-requests, then approvals, then plain comments, else Pending.
// An approval outranks a comment. Those opinions are read from both
// latestOpinionatedReviews and latestReviews, because GitHub drops a reviewer
// from latestReviews once approving clears their review request, so an
// approved-but-unrequired PR would otherwise show as Pending.
func Review(p PR) ReviewState {
	if p.IsDraft {
		return ReviewDraft
	}
	switch p.ReviewDecision {
	case "APPROVED":
		return ReviewApproved
	case "CHANGES_REQUESTED":
		return ReviewChangesRequested
	case "":
		// No required-reviewer rule: derive the state from per-reviewer opinions.
		if hasOpinion(p, "CHANGES_REQUESTED") {
			return ReviewChangesRequested
		}
		if hasOpinion(p, "APPROVED") {
			return ReviewApproved
		}
	}
	// REVIEW_REQUIRED (rule unsatisfied) or empty-with-no-opinion: a plain
	// comment is the most informative state, otherwise Pending.
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

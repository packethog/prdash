// Package pr holds prdash's pure domain types and logic: pull-request models,
// review/CI badge mapping, the merge gate, dedupe, and the refresh backoff.
// It has no dependency on the network, the gh CLI, or the UI.
package pr

import (
	"fmt"
	"time"
)

// Bucket identifies which list a PR belongs to.
type Bucket int

const (
	Authored Bucket = iota
	AwaitingReview
)

// ReviewState is the displayed review status of a PR.
type ReviewState int

const (
	ReviewPending ReviewState = iota
	ReviewChangesRequested
	ReviewApproved
	ReviewDraft
	ReviewCommented
)

func (r ReviewState) String() string {
	switch r {
	case ReviewApproved:
		return "Approved"
	case ReviewChangesRequested:
		return "Changes requested"
	case ReviewDraft:
		return "Draft"
	case ReviewCommented:
		return "Commented"
	default:
		return "Pending review"
	}
}

// CIState is the displayed CI status of a PR.
type CIState int

const (
	CINone CIState = iota
	CIPending
	CISuccess
	CIFailure
)

func (c CIState) Symbol() string {
	switch c {
	case CISuccess:
		return "✓"
	case CIPending:
		return "·"
	case CIFailure:
		return "✗"
	default:
		return "–"
	}
}

// MergeMethod is how a PR is merged.
type MergeMethod int

const (
	MethodSquash MergeMethod = iota
	MethodMerge
	MethodRebase
)

func (m MergeMethod) String() string {
	switch m {
	case MethodMerge:
		return "merge"
	case MethodRebase:
		return "rebase"
	default:
		return "squash"
	}
}

// Next cycles squash -> merge -> rebase -> squash.
func (m MergeMethod) Next() MergeMethod { return (m + 1) % 3 }

// Prev cycles in the opposite direction.
func (m MergeMethod) Prev() MergeMethod { return (m + 2) % 3 }

// PR is one pull request as prdash needs it. Raw GitHub enum strings are kept
// verbatim so badge/gate logic can be tested independently of decoding.
type PR struct {
	Repo             string // owner/name (nameWithOwner)
	Number           int
	Title            string
	URL              string
	HeadRefName      string
	IsDraft          bool
	UpdatedAt        time.Time
	ReviewDecision   string   // "", "REVIEW_REQUIRED", "CHANGES_REQUESTED", "APPROVED"
	Mergeable        string   // "MERGEABLE", "CONFLICTING", "UNKNOWN"
	MergeStateStatus string   // "CLEAN", "BLOCKED", "BEHIND", "DIRTY", "UNSTABLE", "DRAFT", ...
	RollupState      string   // "SUCCESS", "PENDING", "EXPECTED", "FAILURE", "ERROR", ""
	LatestReviews    []string // per-reviewer latest review states: APPROVED, CHANGES_REQUESTED, COMMENTED, DISMISSED, PENDING
	// OpinionatedReviews holds per-reviewer latest APPROVED/CHANGES_REQUESTED
	// states (GitHub's latestOpinionatedReviews). It surfaces an approval that
	// LatestReviews can miss once approving clears the reviewer's request.
	OpinionatedReviews []string
}

// Ref is the short "owner/name#number" identifier.
func (p PR) Ref() string { return fmt.Sprintf("%s#%d", p.Repo, p.Number) }

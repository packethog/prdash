package pr

// MergeBlockers returns the ordered, human-readable reasons a PR fails the merge
// gate. An empty result means the PR may be merged. Order is fixed so the UI and
// tests are deterministic.
func MergeBlockers(p PR) []string {
	var b []string
	if p.IsDraft {
		b = append(b, "PR is a draft")
	}
	if Review(p) != ReviewApproved {
		b = append(b, "review not approved")
	}
	if CI(p) != CISuccess {
		b = append(b, "CI not passing")
	}
	switch p.Mergeable {
	case "MERGEABLE":
		// ok
	case "CONFLICTING":
		b = append(b, "merge conflicts present")
	default:
		b = append(b, "mergeability unknown")
	}
	return b
}

// CanMerge reports whether the PR passes the hard merge gate.
func CanMerge(p PR) bool { return len(MergeBlockers(p)) == 0 }

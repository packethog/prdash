package pr

// DedupeReviewing removes from reviewing any PR whose URL already appears in
// authored, so a PR you both authored and are requested to review shows only
// under Authored.
func DedupeReviewing(authored, reviewing []PR) []PR {
	seen := make(map[string]bool, len(authored))
	for _, p := range authored {
		seen[p.URL] = true
	}
	out := make([]PR, 0, len(reviewing))
	for _, p := range reviewing {
		if !seen[p.URL] {
			out = append(out, p)
		}
	}
	return out
}

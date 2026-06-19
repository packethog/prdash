package gh

import (
	"context"

	"github.com/packethog/prdash/internal/pr"
)

func methodFlag(m pr.MergeMethod) string {
	switch m {
	case pr.MethodMerge:
		return "--merge"
	case pr.MethodRebase:
		return "--rebase"
	default:
		return "--squash"
	}
}

// Merge merges the PR with the given method via `gh pr merge`.
func Merge(ctx context.Context, r Runner, p pr.PR, m pr.MergeMethod, deleteBranch bool) error {
	args := []string{"pr", "merge", p.URL, methodFlag(m)}
	if deleteBranch {
		args = append(args, "--delete-branch")
	}
	_, err := r.Run(ctx, args...)
	return err
}

// Open opens the PR in a browser via `gh pr view --web`.
func Open(ctx context.Context, r Runner, p pr.PR) error {
	_, err := r.Run(ctx, "pr", "view", p.URL, "--web")
	return err
}

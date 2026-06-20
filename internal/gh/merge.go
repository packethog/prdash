package gh

import (
	"context"
	"os"
	"os/exec"
	"runtime"

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

// openArgs is the command used to open a PR URL in a browser. Inside cmux it
// docks an embedded browser pane BELOW the terminal; otherwise it uses gh
// (which opens the default browser).
func openArgs(inCmux bool, url string) (bin string, args []string) {
	if inCmux {
		return "cmux", []string{"new-pane", "--type", "browser", "--direction", "down", "--url", url}
	}
	return "gh", []string{"pr", "view", url, "--web"}
}

// Open opens the PR in a browser. When running inside cmux (CMUX_WORKSPACE_ID is
// set) it opens an embedded browser pane below the terminal; if that fails (e.g.
// cmux is not on PATH) it falls back to `gh pr view --web`.
func Open(ctx context.Context, r Runner, p pr.PR) error {
	if os.Getenv("CMUX_WORKSPACE_ID") != "" {
		bin, args := openArgs(true, p.URL)
		if err := exec.CommandContext(ctx, bin, args...).Run(); err == nil {
			return nil
		} else if ctx.Err() != nil {
			return ctx.Err() // cancelled/timed out: don't spawn the fallback
		}
		// otherwise fall through to gh on cmux failure (e.g. not on PATH)
	}
	_, err := r.Run(ctx, "pr", "view", p.URL, "--web")
	return err
}

// Close closes the PR via `gh pr close`. The branch is kept (no --delete-branch);
// a closed PR can be reopened on GitHub.
func Close(ctx context.Context, r Runner, p pr.PR) error {
	_, err := r.Run(ctx, "pr", "close", p.URL)
	return err
}

// osOpenArgs returns the platform command to open a URL in the default browser.
func osOpenArgs(url string) (bin string, args []string) {
	switch runtime.GOOS {
	case "darwin":
		return "open", []string{url}
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		return "xdg-open", []string{url}
	}
}

// OpenURL opens an arbitrary URL in a browser: a cmux browser pane inside cmux,
// else the OS default opener. Unlike Open (PRs), it never uses `gh pr view --web`
// (which only accepts PR refs).
func OpenURL(ctx context.Context, _ Runner, url string) error {
	if os.Getenv("CMUX_WORKSPACE_ID") != "" {
		bin, args := openArgs(true, url)
		if err := exec.CommandContext(ctx, bin, args...).Run(); err == nil {
			return nil
		} else if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	bin, args := osOpenArgs(url)
	return exec.CommandContext(ctx, bin, args...).Run()
}

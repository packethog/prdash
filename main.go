// Command prdash is a terminal dashboard for your GitHub pull requests:
// the ones you authored and the ones awaiting your review, with review/CI
// badges and gated merge. It shells out to the gh CLI for all GitHub access.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	prconfig "github.com/packethog/prdash/internal/config"
	"github.com/packethog/prdash/internal/gh"
	"github.com/packethog/prdash/internal/ui"
)

type config struct {
	interval time.Duration
	limit    int
}

func parseFlags(args []string) (config, error) {
	fs := flag.NewFlagSet("prdash", flag.ContinueOnError)
	interval := fs.Int("interval", 45, "auto-refresh interval in seconds (min 5)")
	limit := fs.Int("limit", 50, "max PRs fetched per bucket (min 1)")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if *interval < 5 {
		*interval = 5
	}
	if *limit < 1 {
		*limit = 1
	}
	if *limit > 100 {
		*limit = 100 // GraphQL search caps first: at 100
	}
	return config{interval: time.Duration(*interval) * time.Second, limit: *limit}, nil
}

func main() {
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		os.Exit(2)
	}
	rev, ciCfg, prd, err := prconfig.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "prdash:", err) // non-fatal: affected feature stays disabled
	}
	model := ui.New(gh.NewExecRunner(), cfg.interval, cfg.limit, ui.WithReview(rev), ui.WithCI(ciCfg), ui.WithPRDebug(prd))
	if _, err := tea.NewProgram(model, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "prdash:", err)
		os.Exit(1)
	}
}

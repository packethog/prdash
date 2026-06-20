package ui

import (
	"context"
	"fmt"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/packethog/prdash/internal/ci"
	"github.com/packethog/prdash/internal/config"
	"github.com/packethog/prdash/internal/gh"
	"github.com/packethog/prdash/internal/pr"
)

const (
	fetchTimeout    = 20 * time.Second
	mergeTimeout    = 60 * time.Second
	ciListTimeout   = 30 * time.Second
	ciDetailTimeout = 30 * time.Second
	rerunTimeout    = 30 * time.Second
)

type prsFetchedMsg struct{ res gh.FetchResult }
type fetchFailedMsg struct{ err error }
type mergeDoneMsg struct{ p pr.PR }
type mergeFailedMsg struct {
	p   pr.PR
	err error
}
type closeDoneMsg struct{ p pr.PR }
type closeFailedMsg struct {
	p   pr.PR
	err error
}
type openedMsg struct {
	p   pr.PR
	err error
}
type reviewLaunchedMsg struct {
	p   pr.PR
	err error
}
type tickMsg struct{ gen int }

// uiTickMsg fires once per second to keep relative-time displays advancing even
// when there is no user interaction or data fetch.
type uiTickMsg struct{}

type ciFetchedMsg struct {
	gen       int
	workflows []ci.WorkflowRuns
}

type runDetailMsg struct {
	runID int64 // correlation: ignore if it doesn't match the open modal's run
	d     gh.RunDetail
	err   error
}
type summaryMsg struct {
	runID int64
	data  []byte
	err   error
}
type ciDebugLaunchedMsg struct{ err error }
type rerunDoneMsg struct{ run ci.Run }
type rerunFailedMsg struct{ err error }
type openedURLMsg struct{ err error }

func fetchCmd(r gh.Runner, limit int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		res, err := gh.Fetch(ctx, r, limit)
		if err != nil {
			return fetchFailedMsg{err: err}
		}
		return prsFetchedMsg{res: res}
	}
}

func mergeCmd(r gh.Runner, p pr.PR, m pr.MergeMethod) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), mergeTimeout)
		defer cancel()
		if err := gh.Merge(ctx, r, p, m, true); err != nil {
			return mergeFailedMsg{p: p, err: err}
		}
		return mergeDoneMsg{p: p}
	}
}

func closeCmd(r gh.Runner, p pr.PR) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), mergeTimeout)
		defer cancel()
		if err := gh.Close(ctx, r, p); err != nil {
			return closeFailedMsg{p: p, err: err}
		}
		return closeDoneMsg{p: p}
	}
}

func openCmd(r gh.Runner, p pr.PR) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		return openedMsg{p: p, err: gh.Open(ctx, r, p)}
	}
}

func reviewCmd(cmux gh.Runner, rv config.Review, p pr.PR) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		prompt, err := rv.Render(p)
		if err != nil {
			return reviewLaunchedMsg{p: p, err: err}
		}
		return reviewLaunchedMsg{p: p, err: gh.StartReview(ctx, cmux, rv.Provider, rv.Args, prompt)}
	}
}

func tickCmd(d time.Duration, gen int) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return tickMsg{gen: gen} })
}

func uiTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return uiTickMsg{} })
}

// ciFetchCmd fetches all configured workflows concurrently. gen is a generation
// counter: the handler drops the result if a newer fetch has already been
// dispatched (stale-result guard). Per-workflow errors are carried on each
// WorkflowRuns.Err so one bad workflow degrades only its row.
func ciFetchCmd(r gh.Runner, c config.CI, gen int) tea.Cmd {
	return func() tea.Msg {
		wfs := make([]ci.WorkflowRuns, len(c.Workflows))
		var wg sync.WaitGroup
		for i, w := range c.Workflows {
			wg.Add(1)
			go func(i int, w config.Workflow) {
				defer wg.Done()
				// Each goroutine owns a distinct index (wfs[i]); the WaitGroup
				// barrier before the read makes this data-race-free. Recover so a
				// panic in one workflow's fetch degrades its row, not the program.
				defer func() {
					if rec := recover(); rec != nil {
						wfs[i] = ci.WorkflowRuns{Name: w.Name, Repo: w.Repo, Key: w.Workflow, Branch: w.Branch, Err: fmt.Errorf("panic: %v", rec)}
					}
				}()
				ctx, cancel := context.WithTimeout(context.Background(), ciListTimeout)
				defer cancel()
				wr, err := gh.ListRuns(ctx, r, w.Repo, w.Workflow, w.Name, w.Branch, c.LimitFor(w))
				if err != nil {
					wr = ci.WorkflowRuns{Name: w.Name, Repo: w.Repo, Key: w.Workflow, Branch: w.Branch, Err: err}
				}
				wfs[i] = wr
			}(i, w)
		}
		wg.Wait()
		return ciFetchedMsg{gen: gen, workflows: wfs}
	}
}

func runDetailCmd(r gh.Runner, repo string, runID int64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ciDetailTimeout)
		defer cancel()
		d, err := gh.FetchRunDetail(ctx, r, repo, runID)
		return runDetailMsg{runID: runID, d: d, err: err}
	}
}

func summaryCmd(r gh.Runner, repo string, runID int64, glob, file string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ciDetailTimeout)
		defer cancel()
		data, err := gh.FetchRunSummary(ctx, r, repo, runID, glob, file)
		return summaryMsg{runID: runID, data: data, err: err}
	}
}

func ciDebugCmd(cmux gh.Runner, c config.CI, run ci.Run) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		prompt, err := c.Render(config.RunInfo{
			URL:        run.URL,
			Repo:       run.Repo,
			Workflow:   run.WorkflowName,
			Branch:     run.Branch,
			Conclusion: run.Conclusion,
			RunID:      run.RunID,
			RunNumber:  run.RunNumber,
		})
		if err != nil {
			return ciDebugLaunchedMsg{err: err}
		}
		return ciDebugLaunchedMsg{err: gh.StartCIDebug(ctx, cmux, c.Provider, c.Args, prompt)}
	}
}

func rerunCmd(r gh.Runner, run ci.Run) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), rerunTimeout)
		defer cancel()
		if err := gh.RerunFailed(ctx, r, run.Repo, run.RunID); err != nil {
			return rerunFailedMsg{err: err}
		}
		return rerunDoneMsg{run: run}
	}
}

func openURLCmd(r gh.Runner, url string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		return openedURLMsg{err: gh.OpenURL(ctx, r, url)}
	}
}
